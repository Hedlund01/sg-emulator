package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	adminv1 "sg-emulator/gen/admin/v1"
	"sg-emulator/gen/admin/v1/adminv1connect"
	commonv1 "sg-emulator/gen/common/v1"
	currencyv1 "sg-emulator/gen/currency/v1"
	"sg-emulator/gen/currency/v1/currencyv1connect"
	eventv1 "sg-emulator/gen/event/v1"
	"sg-emulator/gen/event/v1/eventv1connect"
	tokenv1 "sg-emulator/gen/token/v1"
	"sg-emulator/gen/token/v1/tokenv1connect"
	"sg-emulator/internal/crypto"
)

// accountCreds holds the on-disk credentials for a test account.
type accountCreds struct {
	id      string
	privKey ed25519.PrivateKey
	pubKey  ed25519.PublicKey
	cert    *x509.Certificate
	certPEM string
}

// newH2CClient returns an HTTP client supporting h2c (unencrypted HTTP/2),
// required for ConnectRPC server-streaming over cleartext connections.
func newH2CClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
}

// newPlainClient returns a plain HTTP/1.1 client suitable for unary RPCs.
func newPlainClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// clients groups all ConnectRPC service clients.
type clients struct {
	currency currencyv1connect.CurrencyServiceClient
	token    tokenv1connect.TokenServiceClient
	event    eventv1connect.EventServiceClient
	admin    adminv1connect.AdminServiceClient
}

// newClients constructs all ConnectRPC clients pointing at addr (host:port).
func newClients(addr string) *clients {
	plain := newPlainClient()
	h2c := newH2CClient()
	base := "http://" + addr
	return &clients{
		currency: currencyv1connect.NewCurrencyServiceClient(plain, base),
		token:    tokenv1connect.NewTokenServiceClient(plain, base),
		event:    eventv1connect.NewEventServiceClient(h2c, base),
		admin:    adminv1connect.NewAdminServiceClient(plain, base),
	}
}

// createTestAccount calls AdminService.CreateAccount and returns parsed credentials.
func createTestAccount(ctx context.Context, admin adminv1connect.AdminServiceClient, initialBalance float64) (*accountCreds, error) {
	resp, err := admin.CreateAccount(ctx, &adminv1.CreateAccountRequest{
		InitialBalance: initialBalance,
	})
	if err != nil {
		return nil, fmt.Errorf("admin CreateAccount RPC: %w", err)
	}
	if !resp.GetSuccess() {
		return nil, fmt.Errorf("create account failed: %s", resp.GetErrorMessage())
	}

	privKey, err := crypto.DecodePrivateKeyPEM([]byte(resp.GetPrivateKeyPem()))
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	pubKey, err := crypto.DecodePublicKeyPEM([]byte(resp.GetPublicKeyPem()))
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	cert, err := crypto.ParseCertificatePEM(resp.GetCertificatePem())
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return &accountCreds{
		id:      resp.GetAccountId(),
		privKey: privKey,
		pubKey:  pubKey,
		cert:    cert,
		certPEM: resp.GetCertificatePem(),
	}, nil
}

// ---------------------------------------------------------------------------
// Signing helpers – mirror the server-side converter expectations.
// Signature.Value in the proto is the raw bytes base64-encoded (std encoding).
// ---------------------------------------------------------------------------

// encodeRawSig base64-encodes raw Ed25519 signature bytes for the proto field.
func encodeRawSig(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}

// tokenIDFromRawSig derives the token ID the same way the domain layer does:
// hex-encode the raw signature bytes that were produced during MintToken signing.
func tokenIDFromRawSig(raw []byte) string {
	return hex.EncodeToString(raw)
}

// toProtoSig converts a domain crypto.Signature to its proto representation.
func toProtoSig(sig *crypto.Signature) *commonv1.Signature {
	return &commonv1.Signature{
		Algorithm: sig.Algorithm,
		Value:     encodeRawSig(sig.Value),
		SignerId:  sig.SignerID,
		Timestamp: sig.Timestamp,
	}
}

// signTransfer builds a signed TransferRequest.
// nonce must equal the sender's current blockchain length + 1.
func signTransfer(from *accountCreds, toID string, amount float64, nonce uint64) (*currencyv1.TransferRequest, error) {
	ts := time.Now().Unix()
	payload := &crypto.TransferPayload{
		From:      from.id,
		To:        toID,
		Amount:    amount,
		Nonce:     nonce,
		Timestamp: ts,
	}
	sig, err := crypto.Sign(payload, from.privKey, from.id)
	if err != nil {
		return nil, err
	}
	return &currencyv1.TransferRequest{
		SignedEnvelope: &currencyv1.SignedTransferEnvelope{
			Payload: &currencyv1.TransferPayload{
				From:      from.id,
				To:        toID,
				Amount:    amount,
				Nonce:     int64(nonce),
				Timestamp: ts,
			},
			Signature:   toProtoSig(sig),
			Certificate: from.certPEM,
		},
	}, nil
}

// signMintToken builds a signed MintTokenRequest.
// It returns the request and the raw signature bytes so the caller can derive
// the token ID via tokenIDFromRawSig.
// nonce must equal the minter's current outgoingTxCount + 1.
func signMintToken(owner *accountCreds, tokenValue string, clawbackAddr string, nonce int64) (*tokenv1.MintTokenRequest, []byte, error) {
	var cbPtr *string
	if clawbackAddr != "" {
		cbPtr = &clawbackAddr
	}
	payload := &crypto.MintTokenPayload{
		TokenValue:      tokenValue,
		ClawbackAddress: cbPtr,
		Nonce:           nonce,
	}
	sig, err := crypto.Sign(payload, owner.privKey, owner.id)
	if err != nil {
		return nil, nil, err
	}
	protoPayload := &tokenv1.MintTokenPayload{
		TokenValue: tokenValue,
		Nonce:      nonce,
	}
	if clawbackAddr != "" {
		protoPayload.ClawbackAddress = clawbackAddr
	}
	return &tokenv1.MintTokenRequest{
		SignedEnvelope: &tokenv1.SignedMintTokenEnvelope{
			Payload:     protoPayload,
			Signature:   toProtoSig(sig),
			Certificate: owner.certPEM,
		},
	}, sig.Value, nil
}

// signLookupToken builds a signed LookupTokenRequest.
func signLookupToken(account *accountCreds, tokenID string) (*tokenv1.LookupTokenRequest, error) {
	payload := &crypto.LookupTokenPayload{TokenID: tokenID, AccountID: account.id}
	sig, err := crypto.Sign(payload, account.privKey, account.id)
	if err != nil {
		return nil, err
	}
	return &tokenv1.LookupTokenRequest{
		SignedEnvelope: &tokenv1.SignedLookupTokenEnvelope{
			Payload:     &tokenv1.LookupTokenPayload{TokenId: tokenID, AccountId: account.id},
			Signature:   toProtoSig(sig),
			Certificate: account.certPEM,
		},
	}, nil
}

// signAuthorizeTokenTransfer builds a signed AuthorizeTokenTransferRequest.
func signAuthorizeTokenTransfer(owner *accountCreds, tokenID string) (*tokenv1.AuthorizeTokenTransferRequest, error) {
	payload := &crypto.AuthorizeTokenTransferPayload{
		AccountID: owner.id,
		TokenID:   tokenID,
	}
	sig, err := crypto.Sign(payload, owner.privKey, owner.id)
	if err != nil {
		return nil, err
	}
	return &tokenv1.AuthorizeTokenTransferRequest{
		SignedEnvelope: &tokenv1.SignedAuthorizeTokenTransferEnvelope{
			Payload: &tokenv1.AuthorizeTokenTransferPayload{
				AccountId: owner.id,
				TokenId:   tokenID,
			},
			Signature:   toProtoSig(sig),
			Certificate: owner.certPEM,
		},
	}, nil
}

// signUnauthorizeTokenTransfer builds a signed UnauthorizeTokenTransferRequest.
func signUnauthorizeTokenTransfer(owner *accountCreds, tokenID string) (*tokenv1.UnauthorizeTokenTransferRequest, error) {
	payload := &crypto.UnauthorizeTokenTransferPayload{
		AccountID: owner.id,
		TokenID:   tokenID,
	}
	sig, err := crypto.Sign(payload, owner.privKey, owner.id)
	if err != nil {
		return nil, err
	}
	return &tokenv1.UnauthorizeTokenTransferRequest{
		SignedEnvelope: &tokenv1.SignedUnauthorizeTokenTransferEnvelope{
			Payload: &tokenv1.UnauthorizeTokenTransferPayload{
				AccountId: owner.id,
				TokenId:   tokenID,
			},
			Signature:   toProtoSig(sig),
			Certificate: owner.certPEM,
		},
	}, nil
}

// signTransferToken builds a signed TransferTokenRequest.
func signTransferToken(from *accountCreds, toID, tokenID string) (*tokenv1.TransferTokenRequest, error) {
	payload := &crypto.TransferTokenPayload{
		From:    from.id,
		To:      toID,
		TokenID: tokenID,
	}
	sig, err := crypto.Sign(payload, from.privKey, from.id)
	if err != nil {
		return nil, err
	}
	return &tokenv1.TransferTokenRequest{
		SignedEnvelope: &tokenv1.SignedTransferTokenEnvelope{
			Payload: &tokenv1.TransferTokenPayload{
				From:    from.id,
				To:      toID,
				TokenId: tokenID,
			},
			Signature:   toProtoSig(sig),
			Certificate: from.certPEM,
		},
	}, nil
}

// signBurnToken builds a signed BurnTokenRequest.
func signBurnToken(owner *accountCreds, tokenID string) (*tokenv1.BurnTokenRequest, error) {
	payload := &crypto.BurnTokenPayload{
		AccountID: owner.id,
		TokenID:   tokenID,
	}
	sig, err := crypto.Sign(payload, owner.privKey, owner.id)
	if err != nil {
		return nil, err
	}
	return &tokenv1.BurnTokenRequest{
		SignedEnvelope: &tokenv1.SignedBurnTokenEnvelope{
			Payload: &tokenv1.BurnTokenPayload{
				AccountId: owner.id,
				TokenId:   tokenID,
			},
			Signature:   toProtoSig(sig),
			Certificate: owner.certPEM,
		},
	}, nil
}

// signClawbackToken builds a signed ClawbackTokenRequest.
// The clawback authority account signs and is the "to" field.
func signClawbackToken(authority *accountCreds, fromID, tokenID string) (*tokenv1.ClawbackTokenRequest, error) {
	payload := &crypto.ClawbackTokenPayload{
		From:    fromID,
		To:      authority.id,
		TokenID: tokenID,
	}
	sig, err := crypto.Sign(payload, authority.privKey, authority.id)
	if err != nil {
		return nil, err
	}
	return &tokenv1.ClawbackTokenRequest{
		SignedEnvelope: &tokenv1.SignedClawbackTokenEnvelope{
			Payload: &tokenv1.ClawbackTokenPayload{
				From:    fromID,
				To:      authority.id,
				TokenId: tokenID,
			},
			Signature:   toProtoSig(sig),
			Certificate: authority.certPEM,
		},
	}, nil
}

// signSubscribe builds a signed SubscribeRequest for the given account.
// Pass nil eventTypes to subscribe to all event types.
func signSubscribe(account *accountCreds, eventTypes []eventv1.EventType) (*eventv1.SubscribeRequest, error) {
	etStrs := make([]string, len(eventTypes))
	for i, et := range eventTypes {
		etStrs[i] = et.String()
	}
	payload := &crypto.SubscribePayload{
		AccountID:  account.id,
		EventTypes: etStrs,
	}
	sig, err := crypto.Sign(payload, account.privKey, account.id)
	if err != nil {
		return nil, err
	}
	return &eventv1.SubscribeRequest{
		SignedEnvelope: &eventv1.SignedSubscribeEnvelope{
			Payload: &eventv1.SubscribePayload{
				AccountId:  account.id,
				EventTypes: eventTypes,
			},
			Signature:   toProtoSig(sig),
			Certificate: account.certPEM,
		},
	}, nil
}

// openSubscription starts a server-streaming Subscribe call in a goroutine
// and returns a channel that receives events and a cancel function to close it.
func openSubscription(
	ctx context.Context,
	c *clients,
	account *accountCreds,
	eventTypes []eventv1.EventType,
) (<-chan *eventv1.Event, context.CancelFunc, error) {
	req, err := signSubscribe(account, eventTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("sign subscribe: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	stream, err := c.event.Subscribe(subCtx, req)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("open stream: %w", err)
	}

	// Wait for the server's initial empty ACK before returning.
	// This ensures any server-side errors (e.g. AlreadyExists for duplicates)
	// are surfaced here rather than being silently swallowed by the goroutine.
	if !stream.Receive() {
		cancel()
		if streamErr := stream.Err(); streamErr != nil {
			return nil, nil, streamErr
		}
		return nil, nil, fmt.Errorf("subscription stream closed before ready ACK")
	}
	// stream.Msg() is the empty initial ACK; discard it.

	ch := make(chan *eventv1.Event, 64)
	go func() {
		defer close(ch)
		for stream.Receive() {
			if ev := stream.Msg().GetEvent(); ev != nil {
				select {
				case ch <- ev:
				case <-subCtx.Done():
					return
				}
			}
		}
	}()

	return ch, cancel, nil
}

// waitForEvent waits up to timeout for an event satisfying the predicate.
func waitForEvent(ch <-chan *eventv1.Event, timeout time.Duration, match func(*eventv1.Event) bool) (*eventv1.Event, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return nil, fmt.Errorf("subscription channel closed before matching event")
			}
			if match(ev) {
				return ev, nil
			}
		case <-deadline.C:
			return nil, fmt.Errorf("timed out waiting for event after %s", timeout)
		}
	}
}

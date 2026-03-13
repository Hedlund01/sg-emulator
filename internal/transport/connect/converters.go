package grpc

import (
	"encoding/base64"

	commonv1 "sg-emulator/gen/common/v1"
	currencyv1 "sg-emulator/gen/currency/v1"
	eventv1 "sg-emulator/gen/event/v1"
	tokenv1 "sg-emulator/gen/token/v1"
	"sg-emulator/internal/crypto"
)

// convertSignature decodes a proto Signature into a domain crypto.Signature,
// base64-decoding the value (standard encoding with URL-safe fallback).
func convertSignature(sig *commonv1.Signature) (crypto.Signature, error) {
	sigBytes, err := base64.StdEncoding.DecodeString(sig.GetValue())
	if err != nil {
		sigBytes, err = base64.URLEncoding.DecodeString(sig.GetValue())
		if err != nil {
			return crypto.Signature{}, err
		}
	}
	return crypto.Signature{
		Algorithm: sig.GetAlgorithm(),
		Value:     sigBytes,
		SignerID:  sig.GetSignerId(),
		Timestamp: sig.GetTimestamp(),
	}, nil
}

// convertTransferEnvelope converts a proto TransferRequest into a domain SignedEnvelope.
func convertTransferEnvelope(req *currencyv1.TransferRequest) (*crypto.SignedEnvelope[*crypto.TransferPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	return &crypto.SignedEnvelope[*crypto.TransferPayload]{
		Payload: &crypto.TransferPayload{
			From:      env.GetPayload().GetFrom(),
			To:        env.GetPayload().GetTo(),
			Amount:    float64(env.GetPayload().GetAmount()),
			Nonce:     uint64(env.GetPayload().GetNonce()),
			Timestamp: env.GetPayload().GetTimestamp(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertMintTokenEnvelope converts a proto MintTokenRequest into a domain SignedEnvelope.
func convertMintTokenEnvelope(req *tokenv1.MintTokenRequest) (*crypto.SignedEnvelope[*crypto.MintTokenPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	var clawbackAddr *string
	if v := env.GetPayload().GetClawbackAddress(); v != "" {
		clawbackAddr = &v
	}
	return &crypto.SignedEnvelope[*crypto.MintTokenPayload]{
		Payload: &crypto.MintTokenPayload{
			TokenValue:      env.GetPayload().GetTokenValue(),
			ClawbackAddress: clawbackAddr,
			Nonce:           env.GetPayload().GetNonce(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertLookupTokenEnvelope converts a proto LookupTokenRequest into a domain SignedEnvelope.
func convertLookupTokenEnvelope(req *tokenv1.LookupTokenRequest) (*crypto.SignedEnvelope[*crypto.LookupTokenPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	return &crypto.SignedEnvelope[*crypto.LookupTokenPayload]{
		Payload: &crypto.LookupTokenPayload{
			TokenID:   env.GetPayload().GetTokenId(),
			AccountID: env.GetPayload().GetAccountId(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertTransferTokenEnvelope converts a proto TransferTokenRequest into a domain SignedEnvelope.
func convertTransferTokenEnvelope(req *tokenv1.TransferTokenRequest) (*crypto.SignedEnvelope[*crypto.TransferTokenPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	return &crypto.SignedEnvelope[*crypto.TransferTokenPayload]{
		Payload: &crypto.TransferTokenPayload{
			From:    env.GetPayload().GetFrom(),
			To:      env.GetPayload().GetTo(),
			TokenID: env.GetPayload().GetTokenId(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertAuthorizeTokenTransferEnvelope converts a proto AuthorizeTokenTransferRequest into a domain SignedEnvelope.
func convertAuthorizeTokenTransferEnvelope(req *tokenv1.AuthorizeTokenTransferRequest) (*crypto.SignedEnvelope[*crypto.AuthorizeTokenTransferPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	return &crypto.SignedEnvelope[*crypto.AuthorizeTokenTransferPayload]{
		Payload: &crypto.AuthorizeTokenTransferPayload{
			AccountID: env.GetPayload().GetAccountId(),
			TokenID:   env.GetPayload().GetTokenId(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertUnauthorizeTokenTransferEnvelope converts a proto UnauthorizeTokenTransferRequest into a domain SignedEnvelope.
func convertUnauthorizeTokenTransferEnvelope(req *tokenv1.UnauthorizeTokenTransferRequest) (*crypto.SignedEnvelope[*crypto.UnauthorizeTokenTransferPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	return &crypto.SignedEnvelope[*crypto.UnauthorizeTokenTransferPayload]{
		Payload: &crypto.UnauthorizeTokenTransferPayload{
			AccountID: env.GetPayload().GetAccountId(),
			TokenID:   env.GetPayload().GetTokenId(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertBurnTokenEnvelope converts a proto BurnTokenRequest into a domain SignedEnvelope.
func convertBurnTokenEnvelope(req *tokenv1.BurnTokenRequest) (*crypto.SignedEnvelope[*crypto.BurnTokenPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	return &crypto.SignedEnvelope[*crypto.BurnTokenPayload]{
		Payload: &crypto.BurnTokenPayload{
			AccountID: env.GetPayload().GetAccountId(),
			TokenID:   env.GetPayload().GetTokenId(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertClawbackTokenEnvelope converts a proto ClawbackTokenRequest into a domain SignedEnvelope.
func convertClawbackTokenEnvelope(req *tokenv1.ClawbackTokenRequest) (*crypto.SignedEnvelope[*crypto.ClawbackTokenPayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}
	return &crypto.SignedEnvelope[*crypto.ClawbackTokenPayload]{
		Payload: &crypto.ClawbackTokenPayload{
			From:    env.GetPayload().GetFrom(),
			To:      env.GetPayload().GetTo(),
			TokenID: env.GetPayload().GetTokenId(),
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

// convertSubscribeEnvelope converts a proto SubscribeRequest into a domain SignedEnvelope.
func convertSubscribeEnvelope(req *eventv1.SubscribeRequest) (*crypto.SignedEnvelope[*crypto.SubscribePayload], error) {
	env := req.GetSignedEnvelope()
	sig, err := convertSignature(env.GetSignature())
	if err != nil {
		return nil, err
	}

	// Convert proto EventType enums to their string representations
	eventTypeStrs := make([]string, 0, len(env.GetPayload().GetEventTypes()))
	for _, et := range env.GetPayload().GetEventTypes() {
		eventTypeStrs = append(eventTypeStrs, et.String())
	}

	return &crypto.SignedEnvelope[*crypto.SubscribePayload]{
		Payload: &crypto.SubscribePayload{
			AccountID:  env.GetPayload().GetAccountId(),
			EventTypes: eventTypeStrs,
		},
		Signature:   sig,
		Certificate: env.GetCertificate(),
	}, nil
}

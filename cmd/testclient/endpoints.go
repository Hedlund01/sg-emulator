package main

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	adminv1 "sg-emulator/gen/admin/v1"
	eventv1 "sg-emulator/gen/event/v1"
)

// endpointResult holds the outcome of a single endpoint test.
type endpointResult struct {
	name    string
	passed  bool
	message string
	elapsed time.Duration
}

func (r endpointResult) String() string {
	status := "PASS"
	if !r.passed {
		status = "FAIL"
	}
	return fmt.Sprintf("  [%s] %-55s (%s) %s", status, r.name, r.elapsed.Round(time.Millisecond), r.message)
}

// RunEndpointTests exercises every ConnectRPC endpoint against the server at
// addr. It creates two fresh accounts via AdminService.CreateAccount so no
// pre-existing accounts on disk are required.
//
// Test order and dependencies:
//  0. AdminService/CreateAccount  – create acc0 and acc1
//  1. AdminService/Mint           – mint into acc0
//  2. Transfer        – account[0] → account[1]
//  3. MintToken       – account[0] mints a token; event stream verifies
//     EVENT_TYPE_MINT_TOKEN arrives on account[0]'s subscription
//  4. AuthorizeTokenTransfer  – account[0] authorises the token for transfer
//  5. TransferToken   – account[0] → account[1]
//  6. UnauthorizeTokenTransfer – account[1] revokes (needs a fresh mint)
//  7. BurnToken       – account[0] burns a fresh token
//  8. ClawbackToken   – mint with clawback=account[1]; account[1] claws back
//  9. Subscribe duplicate – second Subscribe from same account → AlreadyExists
func RunEndpointTests(ctx context.Context, cfg *config) []endpointResult {
	var results []endpointResult

	pass := func(name string, start time.Time) {
		results = append(results, endpointResult{name: name, passed: true, elapsed: time.Since(start)})
	}
	fail := func(name string, start time.Time, err error) {
		results = append(results, endpointResult{
			name:    name,
			passed:  false,
			message: err.Error(),
			elapsed: time.Since(start),
		})
	}

	c := newClients(cfg.addr)

	// ------------------------------------------------------------------
	// 0. Create two fresh accounts via AdminService
	// ------------------------------------------------------------------
	acc0, err := createTestAccount(ctx, c.admin, 100.0)
	if err != nil {
		results = append(results, endpointResult{name: "AdminService/CreateAccount (acc0)", passed: false, message: err.Error()})
		return results
	}
	results = append(results, endpointResult{name: "AdminService/CreateAccount (acc0)", passed: true})

	acc1, err := createTestAccount(ctx, c.admin, 100.0)
	if err != nil {
		results = append(results, endpointResult{name: "AdminService/CreateAccount (acc1)", passed: false, message: err.Error()})
		return results
	}
	results = append(results, endpointResult{name: "AdminService/CreateAccount (acc1)", passed: true})

	// ------------------------------------------------------------------
	// 1. AdminService/Mint
	// ------------------------------------------------------------------
	{
		name := "AdminService/Mint"
		start := time.Now()
		resp, err := c.admin.Mint(ctx, &adminv1.MintRequest{
			AccountId: acc0.id, Amount: 10.0,
		})
		if err != nil {
			fail(name, start, err)
		} else if !resp.GetSuccess() {
			fail(name, start, fmt.Errorf("%s", resp.GetErrorMessage()))
		} else {
			pass(name, start)
		}
	}

	// ------------------------------------------------------------------
	// 2. Transfer
	// ------------------------------------------------------------------
	{
		name := "CurrencyService/Transfer"
		start := time.Now()
		// nonce=0 is correct when the account has no prior transfers; the server
		// will return an error if the nonce is wrong and we surface it as a failure.
		req, err := signTransfer(acc0, acc1.id, 0.01, 0)
		if err != nil {
			fail(name, start, fmt.Errorf("sign: %w", err))
		} else {
			resp, err := c.currency.Transfer(ctx, req)
			if err != nil {
				fail(name, start, err)
			} else if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
			} else {
				pass(name, start)
			}
		}
	}

	// acc0Nonce tracks acc0's current outgoingTxCount (= the nonce for the next tx).
	// After the Transfer above (which used nonce=0), it is now 1.
	var acc0Nonce int64 = 1

	// ------------------------------------------------------------------
	// 2. Subscribe + MintToken (event delivery check)
	//
	// Flow:
	//   a) Open Subscribe stream for acc0 filtering on MINT_TOKEN events.
	//   b) Call MintToken as acc0.
	//   c) Assert the MINT_TOKEN event arrives on the stream with matching token ID.
	//   d) Cancel the subscription.
	// ------------------------------------------------------------------
	var mintedTokenID string // used by subsequent token tests
	{
		name := "TokenService/MintToken + EventService/Subscribe (MINT_TOKEN event)"
		start := time.Now()

		// a) Open subscription before minting so we don't miss the event.
		evCh, cancelSub, err := openSubscription(ctx, c, acc0,
			[]eventv1.EventType{eventv1.EventType_EVENT_TYPE_MINT_TOKEN})
		if err != nil {
			fail(name, start, fmt.Errorf("open subscription: %w", err))
			goto afterMint
		}

		// b) Mint a token.
		{
			req, rawSig, err := signMintToken(acc0, "test-token-v1", "", acc0Nonce)
			acc0Nonce++
			if err != nil {
				cancelSub()
				fail(name, start, fmt.Errorf("sign mint: %w", err))
				goto afterMint
			}
			mintedTokenID = tokenIDFromRawSig(rawSig)

			resp, err := c.token.MintToken(ctx, req)
			if err != nil {
				cancelSub()
				fail(name, start, err)
				goto afterMint
			}
			if !resp.GetSuccess() {
				cancelSub()
				fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
				goto afterMint
			}
		}

		// c) Wait for the MINT_TOKEN event carrying the expected token ID.
		{
			expectedTokenID := mintedTokenID
			ev, err := waitForEvent(evCh, 10*time.Second, func(e *eventv1.Event) bool {
				mt := e.GetMintToken()
				return mt != nil && mt.GetTokenId() == expectedTokenID
			})
			cancelSub()
			if err != nil {
				fail(name, start, fmt.Errorf("event not received: %w (token_id=%s)", err, expectedTokenID))
			} else {
				pass(name, start)
				_ = ev // assertion passed
			}
		}
	}
afterMint:

	// ------------------------------------------------------------------
	// 3. AuthorizeTokenTransfer
	//    The RECEIVER (acc1) must pre-authorize accepting the token before
	//    the sender can transfer it.
	// ------------------------------------------------------------------
	if mintedTokenID != "" {
		name := "TokenService/AuthorizeTokenTransfer"
		start := time.Now()
		req, err := signAuthorizeTokenTransfer(acc1, mintedTokenID)
		if err != nil {
			fail(name, start, fmt.Errorf("sign: %w", err))
		} else {
			resp, err := c.token.AuthorizeTokenTransfer(ctx, req)
			if err != nil {
				fail(name, start, err)
			} else if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
			} else {
				pass(name, start)
			}
		}

		// ------------------------------------------------------------------
		// 4. TransferToken  (requires authorization from step 3)
		// ------------------------------------------------------------------
		{
			name := "TokenService/TransferToken"
			start := time.Now()
			req, err := signTransferToken(acc0, acc1.id, mintedTokenID)
			if err != nil {
				fail(name, start, fmt.Errorf("sign: %w", err))
			} else {
				resp, err := c.token.TransferToken(ctx, req)
				if err != nil {
					fail(name, start, err)
				} else if !resp.GetSuccess() {
					fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
				} else {
					pass(name, start)
				}
			}
		}
	}

	// ------------------------------------------------------------------
	// 5. UnauthorizeTokenTransfer
	//    Mint a fresh token, authorize it, then revoke the authorization.
	// ------------------------------------------------------------------
	{
		name := "TokenService/UnauthorizeTokenTransfer"
		start := time.Now()
		req2, rawSig2, err := signMintToken(acc0, "test-token-unauth", "", acc0Nonce)
		acc0Nonce++
		if err != nil {
			fail(name, start, fmt.Errorf("mint for unauth: %w", err))
			goto afterUnauth
		}
		{
			resp, err := c.token.MintToken(ctx, req2)
			if err != nil {
				fail(name, start, fmt.Errorf("mint rpc: %w", err))
				goto afterUnauth
			}
			if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("mint server error: %s", resp.GetErrorMessage()))
				goto afterUnauth
			}
		}
		unauthTokenID := tokenIDFromRawSig(rawSig2)
		{
			authReq, err := signAuthorizeTokenTransfer(acc1, unauthTokenID)
			if err != nil {
				fail(name, start, fmt.Errorf("sign authorize: %w", err))
				goto afterUnauth
			}
			resp, err := c.token.AuthorizeTokenTransfer(ctx, authReq)
			if err != nil {
				fail(name, start, fmt.Errorf("authorize rpc: %w", err))
				goto afterUnauth
			}
			if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("authorize server error: %s", resp.GetErrorMessage()))
				goto afterUnauth
			}
		}
		{
			unauthReq, err := signUnauthorizeTokenTransfer(acc1, unauthTokenID)
			if err != nil {
				fail(name, start, fmt.Errorf("sign: %w", err))
				goto afterUnauth
			}
			resp, err := c.token.UnauthorizeTokenTransfer(ctx, unauthReq)
			if err != nil {
				fail(name, start, err)
			} else if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
			} else {
				pass(name, start)
			}
		}
	}
afterUnauth:

	// ------------------------------------------------------------------
	// 6. BurnToken  – mint a fresh token then burn it
	// ------------------------------------------------------------------
	{
		name := "TokenService/BurnToken"
		start := time.Now()
		req3, rawSig3, err := signMintToken(acc0, "test-token-burn", "", acc0Nonce)
		acc0Nonce++
		if err != nil {
			fail(name, start, fmt.Errorf("mint for burn: %w", err))
			goto afterBurn
		}
		{
			resp, err := c.token.MintToken(ctx, req3)
			if err != nil {
				fail(name, start, fmt.Errorf("mint rpc: %w", err))
				goto afterBurn
			}
			if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("mint server error: %s", resp.GetErrorMessage()))
				goto afterBurn
			}
		}
		burnTokenID := tokenIDFromRawSig(rawSig3)
		{
			burnReq, err := signBurnToken(acc0, burnTokenID)
			if err != nil {
				fail(name, start, fmt.Errorf("sign: %w", err))
				goto afterBurn
			}
			resp, err := c.token.BurnToken(ctx, burnReq)
			if err != nil {
				fail(name, start, err)
			} else if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
			} else {
				pass(name, start)
			}
		}
	}
afterBurn:

	// ------------------------------------------------------------------
	// 7. ClawbackToken
	//    acc0 mints a token with clawbackAddress=acc1; acc1 claws it back.
	// ------------------------------------------------------------------
	{
		name := "TokenService/ClawbackToken"
		start := time.Now()
		req4, rawSig4, err := signMintToken(acc0, "test-token-clawback", acc1.id, acc0Nonce)
		acc0Nonce++
		if err != nil {
			fail(name, start, fmt.Errorf("mint for clawback: %w", err))
			goto afterClawback
		}
		{
			resp, err := c.token.MintToken(ctx, req4)
			if err != nil {
				fail(name, start, fmt.Errorf("mint rpc: %w", err))
				goto afterClawback
			}
			if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("mint server error: %s", resp.GetErrorMessage()))
				goto afterClawback
			}
		}
		cbTokenID := tokenIDFromRawSig(rawSig4)
		{
			cbReq, err := signClawbackToken(acc1, acc0.id, cbTokenID)
			if err != nil {
				fail(name, start, fmt.Errorf("sign: %w", err))
				goto afterClawback
			}
			resp, err := c.token.ClawbackToken(ctx, cbReq)
			if err != nil {
				fail(name, start, err)
			} else if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
			} else {
				pass(name, start)
			}
		}
	}
afterClawback:

	// ------------------------------------------------------------------
	// 8. LookupToken – look up mintedTokenID on acc1 (transferred there in step 4)
	// ------------------------------------------------------------------
	if mintedTokenID != "" {
		name := "TokenService/LookupToken"
		start := time.Now()
		req, err := signLookupToken(acc1, mintedTokenID)
		if err != nil {
			fail(name, start, fmt.Errorf("sign: %w", err))
		} else {
			resp, err := c.token.LookupToken(ctx, req)
			if err != nil {
				fail(name, start, err)
			} else if !resp.GetSuccess() {
				fail(name, start, fmt.Errorf("server error: %s", resp.GetErrorMessage()))
			} else if resp.GetToken() == nil {
				fail(name, start, fmt.Errorf("expected token in response, got nil"))
			} else if resp.GetToken().GetTokenId() != mintedTokenID {
				fail(name, start, fmt.Errorf("token ID mismatch: got %s, want %s", resp.GetToken().GetTokenId(), mintedTokenID))
			} else {
				pass(name, start)
			}
		}
	}

	// ------------------------------------------------------------------
	// 9. Subscribe duplicate – second call from same account must fail
	// ------------------------------------------------------------------
	{
		name := "EventService/Subscribe (duplicate → AlreadyExists)"
		start := time.Now()

		_, cancel1, err := openSubscription(ctx, c, acc0, nil)
		if err != nil {
			fail(name, start, fmt.Errorf("first subscription failed: %w", err))
			goto afterDuplicate
		}
		defer cancel1()

		// Give the first stream a moment to register on the server.
		time.Sleep(50 * time.Millisecond)

		_, cancel2, err := openSubscription(ctx, c, acc0, nil)
		if err != nil {
			// We expect an error — check it's AlreadyExists.
			if connect.CodeOf(err) == connect.CodeAlreadyExists {
				pass(name, start)
			} else {
				fail(name, start, fmt.Errorf("expected AlreadyExists, got: %w", err))
			}
		} else {
			cancel2()
			fail(name, start, fmt.Errorf("expected AlreadyExists error, but second subscription succeeded"))
		}
	}
afterDuplicate:

	return results
}

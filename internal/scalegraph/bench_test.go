package scalegraph

import (
	"encoding/pem"
	"testing"

	sgcrypto "sg-emulator/internal/crypto"
)

func BenchmarkMintToken(b *testing.B) {
	b.ReportAllocs()

	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(b)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, 0)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		_ = app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: MBR_TOKEN_COST})
		nonce := int64(acc.GetNonce())
		payload := &sgcrypto.MintTokenPayload{TokenValue: "bench-token", Nonce: nonce}
		envelope, envErr := sgcrypto.CreateSignedEnvelope(payload, privKey, acc.ID().String(), certPEM)
		if envErr != nil {
			b.Fatal(envErr)
		}
		b.StartTimer()

		_, err := app.MintToken(testCtx(), &MintTokenRequest{
			TokenValue:     "bench-token",
			Nonce:          nonce,
			SignedEnvelope: envelope,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransfer(b *testing.B) {
	b.ReportAllocs()

	app := testApp()
	acc1 := createTestAccountInApp(b, app, 1e9)
	acc2 := createTestAccountInApp(b, app, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		nonce := acc1.GetNonce()
		b.StartTimer()

		_, err := app.Transfer(testCtx(), &TransferRequest{
			From:   acc1.ID(),
			To:     acc2.ID(),
			Amount: 0.01,
			Nonce:  nonce,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransferToken(b *testing.B) {
	b.ReportAllocs()

	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(b)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, 0)
	if err != nil {
		b.Fatal(err)
	}
	receiver := createTestAccountInApp(b, app, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Mint a fresh token on holder each iteration
		_ = app.Mint(testCtx(), &MintRequest{To: holder.ID(), Amount: MBR_TOKEN_COST})
		mintNonce := int64(holder.GetNonce())
		payload := &sgcrypto.MintTokenPayload{TokenValue: "bench-token", Nonce: mintNonce}
		envelope, envErr := sgcrypto.CreateSignedEnvelope(payload, privKey, holder.ID().String(), certPEM)
		if envErr != nil {
			b.Fatal(envErr)
		}
		mintResp, mintErr := app.MintToken(testCtx(), &MintTokenRequest{
			TokenValue:     "bench-token",
			Nonce:          mintNonce,
			SignedEnvelope: envelope,
		})
		if mintErr != nil {
			b.Fatal(mintErr)
		}
		// Pre-authorize receiver for this token
		_ = app.Mint(testCtx(), &MintRequest{To: receiver.ID(), Amount: MBR_SLOT_COST})
		_ = app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
			AccountID:    receiver.ID(),
			TokenOwnerID: holder.ID(),
			TokenId:      mintResp.TokenID,
			Nonce:        receiver.GetNonce(),
		})
		transferNonce := holder.GetNonce()
		b.StartTimer()

		err := app.TransferToken(testCtx(), &TransferTokenRequest{
			From:    holder.ID(),
			To:      receiver.ID(),
			TokenId: mintResp.TokenID,
			Nonce:   transferNonce,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuthorizeTokenTransfer(b *testing.B) {
	b.ReportAllocs()

	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(b)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, 0)
	if err != nil {
		b.Fatal(err)
	}
	authorizer := createTestAccountInApp(b, app, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Mint a fresh token on holder each iteration
		_ = app.Mint(testCtx(), &MintRequest{To: holder.ID(), Amount: MBR_TOKEN_COST})
		mintNonce := int64(holder.GetNonce())
		payload := &sgcrypto.MintTokenPayload{TokenValue: "bench-token", Nonce: mintNonce}
		envelope, envErr := sgcrypto.CreateSignedEnvelope(payload, privKey, holder.ID().String(), certPEM)
		if envErr != nil {
			b.Fatal(envErr)
		}
		mintResp, mintErr := app.MintToken(testCtx(), &MintTokenRequest{
			TokenValue:     "bench-token",
			Nonce:          mintNonce,
			SignedEnvelope: envelope,
		})
		if mintErr != nil {
			b.Fatal(mintErr)
		}
		_ = app.Mint(testCtx(), &MintRequest{To: authorizer.ID(), Amount: MBR_SLOT_COST})
		authorizerNonce := authorizer.GetNonce()
		b.StartTimer()

		err := app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
			AccountID:    authorizer.ID(),
			TokenOwnerID: holder.ID(),
			TokenId:      mintResp.TokenID,
			Nonce:        authorizerNonce,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

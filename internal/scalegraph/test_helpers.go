package scalegraph

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"sync"
	"testing"
	"time"

	sgcrypto "sg-emulator/internal/crypto"

	"github.com/stretchr/testify/require"
)

// testLogger creates a logger configured for testing
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// testCtx returns a background context for tests
func testCtx() context.Context {
	return context.Background()
}

// testApp creates a new App instance with a test logger
func testApp() *App {
	return NewApp(testLogger())
}

// testKeyPairAndCert generates a fresh Ed25519 key pair and a self-signed test certificate.
// This avoids needing a real CA for unit tests.
func testKeyPairAndCert(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, *x509.Certificate) {
	t.Helper()

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err, "failed to generate key pair")

	// Create a minimal self-signed certificate for testing
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "test-account",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, privKey)
	require.NoError(t, err, "failed to create test certificate")

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err, "failed to parse test certificate")

	return pubKey, privKey, cert
}

// testCreateAccount creates a test account with a generated key pair and certificate.
// Returns the account and the key pair for further use.
func testCreateAccount(t *testing.T) (*Account, *sgcrypto.KeyPair) {
	t.Helper()

	pubKey, privKey, cert := testKeyPairAndCert(t)

	acc, err := newAccountWithPublicKey(pubKey, cert)
	require.NoError(t, err, "failed to create test account")

	kp := &sgcrypto.KeyPair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}

	return acc, kp
}

// testCreateTwoAccounts creates two test accounts for transfer testing.
func testCreateTwoAccounts(t *testing.T) (*Account, *Account) {
	t.Helper()

	acc1, _ := testCreateAccount(t)
	acc2, _ := testCreateAccount(t)

	return acc1, acc2
}

// createTestAccountInApp creates an account in the app with generated credentials and initial balance.
// This replaces the old app.CreateAccount(ctx, balance) pattern.
func createTestAccountInApp(t *testing.T, app *App, balance float64) *Account {
	t.Helper()

	pubKey, _, cert := testKeyPairAndCert(t)

	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, balance)
	require.NoError(t, err, "failed to create test account in app")

	return acc
}

// getTransactionAmount safely extracts the amount from any transaction type.
// The ITransaction interface doesn't include Amount(), so we type-assert to concrete types.
func getTransactionAmount(tx ITransaction) float64 {
	switch t := tx.(type) {
	case *MintTransaction:
		return t.Amount()
	case *TransferTransaction:
		return t.Amount()
	case *BurnTransaction:
		return t.Amount()
	default:
		return 0
	}
}

// runConcurrent runs fn concurrently n times and waits for all goroutines to finish.
func runConcurrent(t *testing.T, n int, fn func(i int)) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			fn(idx)
		}(i)
	}
	wg.Wait()
}

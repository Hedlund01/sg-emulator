# Adding a New Transaction & Request

This guide walks through adding a new transaction type with a corresponding server request, using a **Burn** operation as a concrete example. A burn destroys funds in an account and requires the account owner's signature.

The pipeline has five layers. Each layer has a single file to touch:

```
crypto/signer.go          -- signable data type (if signed)
scalegraph/               -- transaction, request/response, app method, account logic
server/server.go          -- handler registration
server/client.go          -- client convenience method
```

---

## Step 1: Add the transaction type (if new)

If your operation needs a new on-chain transaction, create it in `internal/scalegraph/`.

### 1a. Add the enum value in `transaction.go`

```go
const (
    Transfer TransactionType = iota
    Mint
    Burn     // <-- already exists; add yours here if new
    // ...
)
```

Update the `String()` method to handle the new case.

### 1b. Create the transaction struct

Create `internal/scalegraph/burn_transaction.go`:

```go
package scalegraph

type BurnTransaction struct {
    id       ScalegraphId
    sender   *Account
    receiver *Account
    amount   float64
}

func newBurnTransaction(receiver *Account, amount float64) *BurnTransaction {
    txId, _ := NewScalegraphId()
    return &BurnTransaction{
        id:       txId,
        sender:   nil,       // burns have no sender
        receiver: receiver,
        amount:   amount,
    }
}

// Implement ITransaction: ID(), Type(), Sender(), Receiver()
func (t *BurnTransaction) ID() ScalegraphId      { return t.id }
func (t *BurnTransaction) Type() TransactionType  { return Burn }
func (t *BurnTransaction) Sender() *Account       { return t.sender }
func (t *BurnTransaction) Receiver() *Account     { return t.receiver }
func (t *BurnTransaction) Amount() float64         { return t.amount }
```

### 1c. Handle the transaction in `account.go`

Add a case to `Account.appendTransaction()`:

```go
case Burn:
    tx := trx.(*BurnTransaction)
    if tx.Receiver() != nil && tx.Receiver().ID() == a.ID() {
        if (a.balance - a.mbr) < tx.Amount() {
            return fmt.Errorf("insufficient balance for burn")
        }
        a.balance -= tx.Amount()
    }
```

---

## Step 2: Define the signable data type (if signed)

If the request requires a cryptographic signature, add a signable data struct in `internal/crypto/signer.go`:

```go
type BurnRequest struct {
    AccountID string  `json:"account_id"`
    Amount    float64 `json:"amount"`
}

func (r *BurnRequest) Bytes() []byte {
    data, _ := json.Marshal(r)
    return data
}
```

This struct is what gets signed into a `SignedEnvelope[*crypto.BurnRequest]`. It must implement the `SignableData` interface (just `Bytes() []byte`).

> **Skip this step** for unsigned requests (e.g. `GetAccountsRequest`, `AccountCountRequest`).

---

## Step 3: Define request/response types in `scalegraph/requests.go`

Add your request and response structs to `internal/scalegraph/requests.go`.

### Signed request

```go
type BurnRequest struct {
    AccountID      ScalegraphId
    Amount         float64
    SignedEnvelope *crypto.SignedEnvelope[*crypto.BurnRequest]
}

type BurnResponse struct{}

func (r *BurnRequest) RequiresSignature() bool { return true }

func (r *BurnRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
    return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, r.AccountID.String(),
        func(signed *crypto.BurnRequest) error {
            if signed.AccountID != r.AccountID.String() {
                return fmt.Errorf("AccountID mismatch")
            }
            if signed.Amount != r.Amount {
                return fmt.Errorf("Amount mismatch")
            }
            return nil
        })
}
```

Key decisions in `Verify()`:
- **Second argument to `VerifyRequest`** (`expectedSignerID`):
  - Pass `r.AccountID.String()` if the account owner must sign.
  - Pass `""` (empty string) if the CA must sign.
- **`verifyData` callback**: Cross-check every field in the unsigned request against the signed envelope payload.

### Unsigned request

For requests that don't require a signature, just define the structs without implementing `Verifiable`:

```go
type SomeQueryRequest struct{}
type SomeQueryResponse struct {
    Result string
}
// No RequiresSignature / Verify methods -- server skips verification
```

---

## Step 4: Add the app method in `scalegraph/app.go`

```go
func (a *App) Burn(ctx context.Context, req *BurnRequest) (*BurnResponse, error) {
    logger := a.logger
    if traceID := trace.GetTraceID(ctx); traceID != "" {
        logger = logger.With("trace_id", traceID)
    }

    a.mu.RLock()
    defer a.mu.RUnlock()

    acc, exists := a.accounts[req.AccountID]
    if !exists {
        return nil, fmt.Errorf("account not found: %s", req.AccountID)
    }

    burnTx := newBurnTransaction(acc, req.Amount)
    if err := acc.appendTransaction(burnTx); err != nil {
        return nil, err
    }

    return &BurnResponse{}, nil
}
```

The app method:
- Reads fields from the request struct (not decomposed args).
- Returns `(*XxxResponse, error)`.
- Contains the business logic; the server handler is a thin wrapper.

---

## Step 5: Register the server handler in `server/server.go`

### 5a. Write the handler method

```go
func (s *Server) handleBurn(ctx context.Context, req *scalegraph.BurnRequest) (*scalegraph.BurnResponse, error) {
    return s.app.Burn(ctx, req)
}
```

Most handlers are one-liners that delegate to the app. If you need server-level logic (like `handleCreateAccount` which calls the CA), add it here.

### 5b. Register it in `registerHandlers()`

```go
func (s *Server) registerHandlers() {
    RegisterHandler(s, s.handleCreateAccount)
    RegisterHandler(s, s.handleGetAccount)
    // ...
    RegisterHandler(s, s.handleBurn) // <-- add this line
}
```

That's it. The `RegisterHandler` generic function infers the request/response types from the handler signature and keys the handler map by `reflect.TypeOf(*scalegraph.BurnRequest)`.

Signature verification is **automatic**: if your request type implements `crypto.Verifiable`, the server calls `Verify()` before dispatching to the handler. No manual verification code needed.

---

## Step 6: Add the client convenience method in `server/client.go`

```go
func (c *Client) Burn(ctx context.Context, req *scalegraph.BurnRequest) (*scalegraph.BurnResponse, error) {
    return Send[scalegraph.BurnRequest, scalegraph.BurnResponse](c, ctx, req)
}
```

The generic `Send[Req, Resp]` handles:
- Generating a request ID
- Sending through the channel
- Waiting for the response
- Type-asserting the response payload

For a friendlier API, you can add a wrapper that constructs the request from individual args:

```go
func (c *Client) BurnSigned(ctx context.Context, accountID scalegraph.ScalegraphId, amount float64, signed *crypto.SignedEnvelope[*crypto.BurnRequest]) error {
    _, err := Send[scalegraph.BurnRequest, scalegraph.BurnResponse](c, ctx, &scalegraph.BurnRequest{
        AccountID:      accountID,
        Amount:         amount,
        SignedEnvelope: signed,
    })
    return err
}
```

---

## Summary checklist

| # | File | What to do |
|---|------|------------|
| 1 | `scalegraph/transaction.go` | Add enum value + `String()` case (if new transaction type) |
| 2 | `scalegraph/<name>_transaction.go` | Create transaction struct implementing `ITransaction` |
| 3 | `scalegraph/account.go` | Add case to `appendTransaction()` |
| 4 | `crypto/signer.go` | Add signable data struct with `Bytes()` (if signed) |
| 5 | `scalegraph/requests.go` | Define `XxxRequest` / `XxxResponse`; implement `Verifiable` if signed |
| 6 | `scalegraph/app.go` | Add app method: `(a *App) Xxx(ctx, *XxxRequest) (*XxxResponse, error)` |
| 7 | `server/server.go` | Add handler method + register in `registerHandlers()` |
| 8 | `server/client.go` | Add client convenience method using `Send[Req, Resp]` |

**No enum to update. No switch case in the server. Signature verification is automatic.**

---

## Signer identity reference

| Who signs | `expectedSignerID` in `VerifyRequest` | Example |
|-----------|---------------------------------------|---------|
| Account owner | `r.AccountID.String()` | `TransferRequest`, `GetAccountRequest` |
| CA (system) | `""` (empty string) | `CreateAccountRequest` |
| Derived from envelope | Read from `r.SignedEnvelope.Signature.SignerID` | `MintTokenRequest` |
| Nobody (unsigned) | Don't implement `Verifiable` | `GetAccountsRequest`, `AccountCountRequest` |

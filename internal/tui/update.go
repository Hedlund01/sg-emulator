package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// refreshAccounts fetches all accounts, sorts them by ID, and stores the
// result in m.cachedAccounts. Call this on every view entry and after any
// mutating action so that Update and View always share the same ordered slice.
func (m *Model) refreshAccounts() error {
	accs, err := m.app.GetAccounts(context.Background())
	if err != nil {
		return err
	}
	sort.Slice(accs, func(i, j int) bool {
		return accs[i].ID().String() < accs[j].ID().String()
	})
	m.cachedAccounts = accs
	m.cachedTokens = nil // invalidate token cache whenever accounts refresh
	return nil
}

// refreshTokens fetches tokens for the account at the given index into
// cachedAccounts, sorts them by ID, and stores the result in m.cachedTokens.
func (m *Model) refreshTokens() error {
	return m.refreshTokensForIndex(m.tokenAccountIndex)
}

// refreshTokensForIndex fetches tokens for the account at the given index.
func (m *Model) refreshTokensForIndex(idx int) error {
	if idx >= len(m.cachedAccounts) {
		m.cachedTokens = nil
		return nil
	}
	toks := m.cachedAccounts[idx].GetTokens()
	sort.Slice(toks, func(i, j int) bool {
		return toks[i].ID() < toks[j].ID()
	})
	m.cachedTokens = toks
	return nil
}

// Update handles incoming messages and updates the model accordingly
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Handle based on current view
		switch m.view {
		case ViewMenu:
			return m.updateMenu(msg)
		case ViewCreateAccount:
			return m.updateCreateAccount(msg)
		case ViewAccountDetail:
			return m.updateAccountDetail(msg)
		case ViewAccountDetailSingle:
			return m.updateAccountDetailSingle(msg)
		case ViewTransactionDetail:
			return m.updateTransactionDetail(msg)
		case ViewSendMoney:
			return m.updateSendMoney(msg)
		case ViewVirtualNodes:
			return m.updateVirtualNodes(msg)
		case ViewTokenMenu:
			return m.updateTokenMenu(msg)
		case ViewMintToken:
			return m.updateMintToken(msg)
		case ViewAuthorizeTokenTransfer:
			return m.updateAuthorizeTokenTransfer(msg)
		case ViewUnauthorizeTokenTransfer:
			return m.updateUnauthorizeTokenTransfer(msg)
		case ViewTransferToken:
			return m.updateTransferToken(msg)
		case ViewTokenList:
			return m.updateTokenList(msg)
		case ViewBurnToken:
			return m.updateBurnToken(msg)
		case ViewClawbackToken:
			return m.updateClawbackToken(msg)
		case ViewFreezeToken:
			return m.updateFreezeToken(msg)
		case ViewUnfreezeToken:
			return m.updateUnfreezeToken(msg)
		case ViewLookupToken:
			return m.updateLookupToken(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}

	return m, nil
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(menuItems)-1 {
			m.menuCursor++
		}
	case "enter":
		m.statusMsg = ""
		switch m.menuCursor {
		case 0: // Create Account
			m.view = ViewCreateAccount
			m.createAccountFocus = 0
			m.balanceInput.SetValue("")
			m.nameInput.SetValue("")
			m.balanceInput.Focus()
			m.balanceInput.PromptStyle = focusedLabelStyle
			m.balanceInput.TextStyle = focusedLabelStyle
			m.nameInput.Blur()
			m.nameInput.PromptStyle = blurredLabelStyle
			m.nameInput.TextStyle = blurredLabelStyle
		case 1: // View Account
			if err := m.refreshAccounts(); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.view = ViewAccountDetail
			m.selectedAccountIndex = 0
		case 2: // Send Money
			if err := m.refreshAccounts(); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.view = ViewSendMoney
			m.sendStep = 0
			m.sendFromIndex = 0
			m.sendToIndex = 0
			m.sendAmount = ""
		case 3: // View Virtual Nodes
			m.view = ViewVirtualNodes
			m.sendAmount = ""
		case 4: // Token Operations
			if err := m.refreshAccounts(); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.view = ViewTokenMenu
			m.tokenMenuCursor = 0
		}
	}
	return m, nil
}

func (m Model) updateCreateAccount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.view = ViewMenu
		m.balanceInput.Blur()
		m.nameInput.Blur()
		m.statusMsg = ""
		return m, nil

	case "tab", "shift+tab":
		// Cycle through inputs
		if msg.String() == "tab" {
			m.createAccountFocus++
			if m.createAccountFocus > 2 {
				m.createAccountFocus = 0
			}
		} else {
			m.createAccountFocus--
			if m.createAccountFocus < 0 {
				m.createAccountFocus = 2
			}
		}

		// Update focus and styles
		switch m.createAccountFocus {
		case 0:
			cmd = m.balanceInput.Focus()
			m.balanceInput.PromptStyle = focusedLabelStyle
			m.balanceInput.TextStyle = focusedLabelStyle
			m.nameInput.Blur()
			m.nameInput.PromptStyle = blurredLabelStyle
			m.nameInput.TextStyle = blurredLabelStyle
		case 1:
			m.balanceInput.Blur()
			m.balanceInput.PromptStyle = blurredLabelStyle
			m.balanceInput.TextStyle = blurredLabelStyle
			cmd = m.nameInput.Focus()
			m.nameInput.PromptStyle = focusedLabelStyle
			m.nameInput.TextStyle = focusedLabelStyle
		default:
			m.balanceInput.Blur()
			m.balanceInput.PromptStyle = blurredLabelStyle
			m.balanceInput.TextStyle = blurredLabelStyle
			m.nameInput.Blur()
			m.nameInput.PromptStyle = blurredLabelStyle
			m.nameInput.TextStyle = blurredLabelStyle
		}
		return m, cmd

	case "enter":
		// If on submit button, create account
		if m.createAccountFocus == 2 {
			balance, err := strconv.ParseFloat(m.balanceInput.Value(), 64)
			if err != nil || balance < 0 {
				m.statusMsg = "Invalid balance"
				return m, nil
			}
			acc, err := m.createAccountWithCA(context.Background(), balance)
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			// Save the name if provided
			if m.nameInput.Value() != "" {
				m.accountNames[acc.ID()] = m.nameInput.Value()
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Account created!"
			m.view = ViewMenu
			m.balanceInput.Blur()
			m.nameInput.Blur()
			return m, nil
		} else {
			// Move to next field on enter
			m.createAccountFocus++
			if m.createAccountFocus > 2 {
				m.createAccountFocus = 0
			}
			// Update focus and styles
			switch m.createAccountFocus {
			case 0:
				cmd = m.balanceInput.Focus()
				m.balanceInput.PromptStyle = focusedLabelStyle
				m.balanceInput.TextStyle = focusedLabelStyle
				m.nameInput.Blur()
				m.nameInput.PromptStyle = blurredLabelStyle
				m.nameInput.TextStyle = blurredLabelStyle
			case 1:
				m.balanceInput.Blur()
				m.balanceInput.PromptStyle = blurredLabelStyle
				m.balanceInput.TextStyle = blurredLabelStyle
				cmd = m.nameInput.Focus()
				m.nameInput.PromptStyle = focusedLabelStyle
				m.nameInput.TextStyle = focusedLabelStyle
			default:
				m.balanceInput.Blur()
				m.balanceInput.PromptStyle = blurredLabelStyle
				m.balanceInput.TextStyle = blurredLabelStyle
				m.nameInput.Blur()
				m.nameInput.PromptStyle = blurredLabelStyle
				m.nameInput.TextStyle = blurredLabelStyle
			}
			return m, cmd
		}
	}

	// Update the focused input
	switch m.createAccountFocus {
	case 0:
		m.balanceInput, cmd = m.balanceInput.Update(msg)
	case 1:
		m.nameInput, cmd = m.nameInput.Update(msg)
	}
	return m, cmd
}

func (m Model) updateAccountDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts
	accountCount := len(accounts)

	switch msg.String() {
	case "esc", "q":
		m.view = ViewMenu
	case "up", "k":
		if m.selectedAccountIndex > 0 {
			m.selectedAccountIndex--
		}
	case "down", "j":
		if m.selectedAccountIndex < accountCount-1 {
			m.selectedAccountIndex++
		}
	case "enter":
		if accountCount > 0 {
			blocks := accounts[m.selectedAccountIndex].Blockchain().GetBlocks()
			txCount := 0
			for _, b := range blocks {
				if b.Transaction() != nil {
					txCount++
				}
			}
			m.selectedTransactionIndex = txCount - 1
			if m.selectedTransactionIndex < 0 {
				m.selectedTransactionIndex = 0
			}
			m.transactionOffset = 0
			m.view = ViewAccountDetailSingle
		}
	}
	return m, nil
}

func (m Model) updateAccountDetailSingle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts
	var txCount int
	if len(accounts) > 0 && m.selectedAccountIndex < len(accounts) {
		blocks := accounts[m.selectedAccountIndex].Blockchain().GetBlocks()
		for _, b := range blocks {
			if b.Transaction() != nil {
				txCount++
			}
		}
	}

	switch msg.String() {
	case "esc", "q":
		m.view = ViewAccountDetail
	case "up", "k":
		if m.selectedTransactionIndex < txCount-1 {
			m.selectedTransactionIndex++
			// Adjust scroll offset if needed
			if m.selectedTransactionIndex > txCount-1-m.transactionOffset {
				if m.transactionOffset < txCount-1 {
					m.transactionOffset++
				}
			}
		}
	case "down", "j":
		if m.selectedTransactionIndex > 0 {
			m.selectedTransactionIndex--
			// Adjust scroll offset if needed
			if m.selectedTransactionIndex < txCount-1-m.transactionOffset-4 {
				if m.transactionOffset > 0 {
					m.transactionOffset--
				}
			}
		}
	case "enter":
		if txCount > 0 {
			m.view = ViewTransactionDetail
		}
	}
	return m, nil
}

func (m Model) updateTransactionDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.view = ViewAccountDetailSingle
	}
	return m, nil
}

func (m Model) updateSendMoney(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts
	accountCount := len(accounts)

	switch msg.String() {
	case "esc":
		if m.sendStep > 0 {
			m.sendStep--
			m.statusMsg = ""
		} else {
			m.view = ViewMenu
		}
	case "up", "k":
		switch m.sendStep {
		case 0:
			if m.sendFromIndex > 0 {
				m.sendFromIndex--
			}
		case 1:
			if m.sendToIndex > 0 {
				// Skip the from account
				m.sendToIndex--
				if m.sendToIndex == m.sendFromIndex {
					if m.sendToIndex > 0 {
						m.sendToIndex--
					} else {
						m.sendToIndex++
					}
				}
			}
		}
	case "down", "j":
		switch m.sendStep {
		case 0:
			if m.sendFromIndex < accountCount-1 {
				m.sendFromIndex++
			}
		case 1:
			if m.sendToIndex < accountCount-1 {
				m.sendToIndex++
				if m.sendToIndex == m.sendFromIndex {
					if m.sendToIndex < accountCount-1 {
						m.sendToIndex++
					} else {
						m.sendToIndex--
					}
				}
			}
		}
	case "enter":
		switch m.sendStep {
		case 0:
			m.sendStep = 1
			// Initialize sendToIndex to first account that isn't sendFromIndex
			m.sendToIndex = 0
			if m.sendToIndex == m.sendFromIndex {
				m.sendToIndex = 1
			}
		case 1:
			m.sendStep = 2
			m.sendAmount = ""
		case 2:
			amount, err := strconv.ParseFloat(m.sendAmount, 64)
			if err != nil || amount <= 0 {
				m.statusMsg = "Invalid amount"
				return m, nil
			}

			ctx := context.Background()
			fromID := accounts[m.sendFromIndex].ID()
			toID := accounts[m.sendToIndex].ID()

			// Get account to calculate nonce
			fromAccount := accounts[m.sendFromIndex]
			nonce := fromAccount.GetNonce()

			// Load credentials (from cache or disk)
			creds, err := m.getCredentials(fromID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}

			// Parse private key
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}

			// Create and sign transfer request
			transferReq := &crypto.TransferPayload{
				From:      fromID.String(),
				To:        toID.String(),
				Amount:    amount,
				Nonce:     nonce,
				Timestamp: time.Now().Unix(),
			}

			signedEnvelope, err := crypto.CreateSignedEnvelope(
				transferReq,
				privKey,
				fromID.String(),
				creds.CertificatePEM,
			)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}

			_, err = m.app.TransferSigned(ctx, signedEnvelope)
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}

			_ = m.refreshAccounts()
			m.statusMsg = "Transfer complete!"
			m.view = ViewMenu
			m.sendStep = 0
		}
	case "backspace":
		if m.sendStep == 2 && len(m.sendAmount) > 0 {
			m.sendAmount = m.sendAmount[:len(m.sendAmount)-1]
		}
	default:
		if m.sendStep == 2 && len(msg.String()) == 1 {
			c := msg.String()[0]
			if (c >= '0' && c <= '9') || c == '.' {
				m.sendAmount += msg.String()
			}
		}
	}
	return m, nil
}

func (m Model) updateVirtualNodes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.view = ViewMenu
		m.statusMsg = ""
	}

	return m, nil
}

// getCredentials loads credentials from CA store, using cache if available
func (m *Model) getCredentials(accountID scalegraph.ScalegraphId) (*AccountCredentials, error) {
	// Check cache first
	if creds, ok := m.credentialCache[accountID]; ok {
		return creds, nil
	}

	// Load from CA store
	ca := m.server.CA()
	if ca == nil {
		return nil, fmt.Errorf("CA not available")
	}

	privKeyPEM, err := ca.GetAccountPrivateKeyPEM(accountID.String())
	if err != nil {
		return nil, err
	}

	certPEM, err := ca.GetAccountCertificatePEM(accountID.String())
	if err != nil {
		return nil, err
	}

	// Cache and return
	creds := &AccountCredentials{
		PrivateKeyPEM:  privKeyPEM,
		CertificatePEM: certPEM,
	}
	m.credentialCache[accountID] = creds
	return creds, nil
}

func (m Model) updateTokenMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const tokenMenuItemCount = 8
	switch msg.String() {
	case "esc":
		m.view = ViewMenu
		m.statusMsg = ""
	case "up", "k":
		if m.tokenMenuCursor > 0 {
			m.tokenMenuCursor--
		}
	case "down", "j":
		if m.tokenMenuCursor < tokenMenuItemCount-1 {
			m.tokenMenuCursor++
		}
	case "enter":
		m.tokenStep = 0
		m.tokenAccountIndex = 0
		m.tokenToAccountIndex = 0
		m.tokenSourceIndex = 0
		m.tokenClawbackIndex = 0
		m.tokenFreezeIndex = 0
		m.tokenTokenIndex = 0
		m.tokenListOffset = 0
		m.statusMsg = ""
		if err := m.refreshAccounts(); err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		switch m.tokenMenuCursor {
		case 0:
			m.view = ViewMintToken
			m.tokenValueInput.SetValue("")
			m.tokenValueInput.Blur()
		case 1:
			m.view = ViewAuthorizeTokenTransfer
		case 2:
			m.view = ViewUnauthorizeTokenTransfer
		case 3:
			m.view = ViewTransferToken
		case 4:
			m.view = ViewTokenList
		case 5:
			m.view = ViewBurnToken
		case 6:
			m.view = ViewClawbackToken
		case 7:
			m.view = ViewFreezeToken
		case 8:
			m.view = ViewUnfreezeToken
		case 9:
			m.view = ViewLookupToken
			m.lookupTokenInput.SetValue("")
			m.lookupTokenInput.Blur()
			m.lookupResult = nil
		}
	}
	return m, nil
}

func (m Model) updateMintToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
			if m.tokenStep == 1 {
				m.tokenValueInput.Focus()
			}
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0:
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			m.tokenStep = 1
			m.tokenValueInput.SetValue("")
			m.tokenValueInput.Focus()
		}
	case 1:
		switch msg.String() {
		case "enter":
			if m.tokenValueInput.Value() == "" {
				m.statusMsg = "Token value cannot be empty"
				return m, nil
			}
			m.tokenStep = 2
			m.tokenClawbackIndex = 0
			m.tokenValueInput.Blur()
		default:
			var cmd tea.Cmd
			m.tokenValueInput, cmd = m.tokenValueInput.Update(msg)
			return m, cmd
		}
	case 2:
		// clawback list size: 1 ("No clawback") + len(accounts)
		clawbackCount := len(accounts) + 1
		switch msg.String() {
		case "up", "k":
			if m.tokenClawbackIndex > 0 {
				m.tokenClawbackIndex--
			}
		case "down", "j":
			if m.tokenClawbackIndex < clawbackCount-1 {
				m.tokenClawbackIndex++
			}
		case "enter":
			m.tokenStep = 3
			m.tokenFreezeIndex = 0
			m.statusMsg = ""
		}
	case 3:
		// freeze list size: 1 ("No freeze") + len(accounts)
		freezeCount := len(accounts) + 1
		switch msg.String() {
		case "up", "k":
			if m.tokenFreezeIndex > 0 {
				m.tokenFreezeIndex--
			}
		case "down", "j":
			if m.tokenFreezeIndex < freezeCount-1 {
				m.tokenFreezeIndex++
			}
		case "enter":
			// Resolve clawback address (nil if index 0)
			var clawbackAddr *scalegraph.ScalegraphId
			if m.tokenClawbackIndex > 0 {
				id := accounts[m.tokenClawbackIndex-1].ID()
				clawbackAddr = &id
			}

			// Resolve freeze address (nil if index 0)
			var freezeAddr *scalegraph.ScalegraphId
			if m.tokenFreezeIndex > 0 {
				id := accounts[m.tokenFreezeIndex-1].ID()
				freezeAddr = &id
			}

			signerID := accounts[m.tokenAccountIndex].ID()
			creds, err := m.getCredentials(signerID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}

			nonce := int64(accounts[m.tokenAccountIndex].GetNonce())
			payload := &crypto.MintTokenPayload{
				TokenValue: m.tokenValueInput.Value(),
				Nonce:      nonce,
			}
			if clawbackAddr != nil {
				clawbackStr := clawbackAddr.String()
				payload.ClawbackAddress = &clawbackStr
			}
			if freezeAddr != nil {
				freezeStr := freezeAddr.String()
				payload.FreezeAddress = &freezeStr
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, signerID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}

			resp, err := m.app.MintTokenSigned(context.Background(), signedEnvelope)
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}

			tokenIDPreview := resp.TokenID
			if len(tokenIDPreview) > 8 {
				tokenIDPreview = tokenIDPreview[:8] + "..."
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token minted! ID: " + tokenIDPreview
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateAuthorizeTokenTransfer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0: // Pick the receiving account
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if len(accounts) < 2 {
				m.statusMsg = "Need at least 2 accounts"
				return m, nil
			}
			m.tokenStep = 1
			// Default source to first account that isn't the receiver
			m.tokenSourceIndex = 0
			if m.tokenSourceIndex == m.tokenAccountIndex {
				m.tokenSourceIndex = 1
			}
			m.statusMsg = ""
		}

	case 1: // Pick the source account (token owner)
		switch msg.String() {
		case "up", "k":
			if m.tokenSourceIndex > 0 {
				m.tokenSourceIndex--
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex > 0 {
						m.tokenSourceIndex--
					} else {
						m.tokenSourceIndex++
					}
				}
			}
		case "down", "j":
			if m.tokenSourceIndex < len(accounts)-1 {
				m.tokenSourceIndex++
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex < len(accounts)-1 {
						m.tokenSourceIndex++
					} else {
						m.tokenSourceIndex--
					}
				}
			}
		case "enter":
			if err := m.refreshTokensForIndex(m.tokenSourceIndex); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			if len(m.cachedTokens) == 0 {
				m.statusMsg = "Selected account has no owned tokens"
				return m, nil
			}
			m.tokenStep = 2
			m.tokenTokenIndex = 0
			m.statusMsg = ""
		}

	case 2: // Pick a token from the source account
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
			}
		case "enter":
			selectedToken := tokens[m.tokenTokenIndex]
			receiverID := accounts[m.tokenAccountIndex].ID()

			creds, err := m.getCredentials(receiverID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			tokenOwnerID := accounts[m.tokenSourceIndex].ID()
			payload := &crypto.AuthorizeTokenTransferPayload{
				AccountID:    receiverID.String(),
				TokenID:      selectedToken.ID(),
				TokenOwnerID: tokenOwnerID.String(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, receiverID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.AuthorizeTokenTransferSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token transfer authorized!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateUnauthorizeTokenTransfer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0: // Pick the account revoking authorization (the receiver)
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if len(accounts) < 2 {
				m.statusMsg = "Need at least 2 accounts"
				return m, nil
			}
			m.tokenStep = 1
			m.tokenSourceIndex = 0
			if m.tokenSourceIndex == m.tokenAccountIndex {
				m.tokenSourceIndex = 1
			}
			m.statusMsg = ""
		}

	case 1: // Pick the source account (token owner) to browse tokens from
		switch msg.String() {
		case "up", "k":
			if m.tokenSourceIndex > 0 {
				m.tokenSourceIndex--
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex > 0 {
						m.tokenSourceIndex--
					} else {
						m.tokenSourceIndex++
					}
				}
			}
		case "down", "j":
			if m.tokenSourceIndex < len(accounts)-1 {
				m.tokenSourceIndex++
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex < len(accounts)-1 {
						m.tokenSourceIndex++
					} else {
						m.tokenSourceIndex--
					}
				}
			}
		case "enter":
			if err := m.refreshTokensForIndex(m.tokenSourceIndex); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			if len(m.cachedTokens) == 0 {
				m.statusMsg = "Selected account has no owned tokens"
				return m, nil
			}
			m.tokenStep = 2
			m.tokenTokenIndex = 0
			m.statusMsg = ""
		}

	case 2: // Pick a token from the source account
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
			}
		case "enter":
			selectedToken := tokens[m.tokenTokenIndex]
			receiverID := accounts[m.tokenAccountIndex].ID()

			creds, err := m.getCredentials(receiverID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			tokenOwnerID := accounts[m.tokenSourceIndex].ID()
			payload := &crypto.UnauthorizeTokenTransferPayload{
				AccountID:    receiverID.String(),
				TokenID:      selectedToken.ID(),
				TokenOwnerID: tokenOwnerID.String(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, receiverID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.UnauthorizeTokenTransferSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token transfer unauthorized!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateTransferToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0:
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if err := m.refreshTokens(); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			if len(m.cachedTokens) == 0 {
				m.statusMsg = "This account has no owned tokens"
				return m, nil
			}
			m.tokenStep = 1
			m.tokenTokenIndex = 0
			m.statusMsg = ""
		}
	case 1:
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
			}
		case "enter":
			m.tokenStep = 2
			m.tokenToAccountIndex = 0
			if m.tokenToAccountIndex == m.tokenAccountIndex {
				m.tokenToAccountIndex = 1
			}
			m.statusMsg = ""
		}
	case 2:
		switch msg.String() {
		case "up", "k":
			if m.tokenToAccountIndex > 0 {
				m.tokenToAccountIndex--
				if m.tokenToAccountIndex == m.tokenAccountIndex {
					if m.tokenToAccountIndex > 0 {
						m.tokenToAccountIndex--
					} else {
						m.tokenToAccountIndex++
					}
				}
			}
		case "down", "j":
			if m.tokenToAccountIndex < len(accounts)-1 {
				m.tokenToAccountIndex++
				if m.tokenToAccountIndex == m.tokenAccountIndex {
					if m.tokenToAccountIndex < len(accounts)-1 {
						m.tokenToAccountIndex++
					} else {
						m.tokenToAccountIndex--
					}
				}
			}
		case "enter":
			fromAcc := accounts[m.tokenAccountIndex]
			toAcc := accounts[m.tokenToAccountIndex]
			selectedToken := m.cachedTokens[m.tokenTokenIndex]
			fromID := fromAcc.ID()
			toID := toAcc.ID()
			creds, err := m.getCredentials(fromID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			payload := &crypto.TransferTokenPayload{
				From:    fromID.String(),
				To:      toID.String(),
				TokenID: selectedToken.ID(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, fromID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.TransferTokenSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token transferred!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateBurnToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0: // Pick the account that owns the token
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if err := m.refreshTokens(); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			if len(m.cachedTokens) == 0 {
				m.statusMsg = "This account has no owned tokens"
				return m, nil
			}
			m.tokenStep = 1
			m.tokenTokenIndex = 0
			m.statusMsg = ""
		}

	case 1: // Pick the token to burn
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
			}
		case "enter":
			ownerAcc := accounts[m.tokenAccountIndex]
			selectedToken := tokens[m.tokenTokenIndex]
			ownerID := ownerAcc.ID()

			creds, err := m.getCredentials(ownerID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			payload := &crypto.BurnTokenPayload{
				AccountID: ownerID.String(),
				TokenID:   selectedToken.ID(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, ownerID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.BurnTokenSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token burned!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateClawbackToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0: // Pick the clawback authority account (To — this account signs)
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if len(accounts) < 2 {
				m.statusMsg = "Need at least 2 accounts"
				return m, nil
			}
			m.tokenSourceIndex = 0
			if m.tokenSourceIndex == m.tokenAccountIndex {
				m.tokenSourceIndex = 1
			}
			m.tokenStep = 1
			m.statusMsg = ""
		}

	case 1: // Pick the token holder account (From)
		switch msg.String() {
		case "up", "k":
			if m.tokenSourceIndex > 0 {
				m.tokenSourceIndex--
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex > 0 {
						m.tokenSourceIndex--
					} else {
						m.tokenSourceIndex++
					}
				}
			}
		case "down", "j":
			if m.tokenSourceIndex < len(accounts)-1 {
				m.tokenSourceIndex++
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex < len(accounts)-1 {
						m.tokenSourceIndex++
					} else {
						m.tokenSourceIndex--
					}
				}
			}
		case "enter":
			// Load tokens for the holder account and filter by clawback authority
			if err := m.refreshTokensForIndex(m.tokenSourceIndex); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			authorityID := accounts[m.tokenAccountIndex].ID()
			var filtered []*scalegraph.Token
			for _, tok := range m.cachedTokens {
				if cb := tok.ClawbackAddress(); cb != nil && *cb == authorityID {
					filtered = append(filtered, tok)
				}
			}
			if len(filtered) == 0 {
				m.statusMsg = "No clawable tokens for this authority in selected account"
				return m, nil
			}
			m.cachedTokens = filtered
			m.tokenTokenIndex = 0
			m.tokenStep = 2
			m.statusMsg = ""
		}

	case 2: // Pick the token to claw back
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
			}
		case "enter":
			authorityAcc := accounts[m.tokenAccountIndex]
			holderAcc := accounts[m.tokenSourceIndex]
			selectedToken := tokens[m.tokenTokenIndex]
			authorityID := authorityAcc.ID()
			holderID := holderAcc.ID()

			creds, err := m.getCredentials(authorityID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			payload := &crypto.ClawbackTokenPayload{
				From:    holderID.String(),
				To:      authorityID.String(),
				TokenID: selectedToken.ID(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, authorityID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.ClawbackTokenSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token clawed back!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateFreezeToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0: // Pick the freeze authority account (signs)
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if len(accounts) < 2 {
				m.statusMsg = "Need at least 2 accounts"
				return m, nil
			}
			m.tokenSourceIndex = 0
			if m.tokenSourceIndex == m.tokenAccountIndex {
				m.tokenSourceIndex = 1
			}
			m.tokenStep = 1
			m.statusMsg = ""
		}

	case 1: // Pick the token holder account
		switch msg.String() {
		case "up", "k":
			if m.tokenSourceIndex > 0 {
				m.tokenSourceIndex--
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex > 0 {
						m.tokenSourceIndex--
					} else {
						m.tokenSourceIndex++
					}
				}
			}
		case "down", "j":
			if m.tokenSourceIndex < len(accounts)-1 {
				m.tokenSourceIndex++
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex < len(accounts)-1 {
						m.tokenSourceIndex++
					} else {
						m.tokenSourceIndex--
					}
				}
			}
		case "enter":
			if err := m.refreshTokensForIndex(m.tokenSourceIndex); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			authorityID := accounts[m.tokenAccountIndex].ID()
			var filtered []*scalegraph.Token
			for _, tok := range m.cachedTokens {
				if fa := tok.FreezeAddress(); fa != nil && *fa == authorityID && !tok.Frozen() {
					filtered = append(filtered, tok)
				}
			}
			if len(filtered) == 0 {
				m.statusMsg = "No freezable tokens for this authority in selected account"
				return m, nil
			}
			m.cachedTokens = filtered
			m.tokenTokenIndex = 0
			m.tokenStep = 2
			m.statusMsg = ""
		}

	case 2: // Pick the token to freeze
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
			}
		case "enter":
			authorityAcc := accounts[m.tokenAccountIndex]
			holderAcc := accounts[m.tokenSourceIndex]
			selectedToken := tokens[m.tokenTokenIndex]
			authorityID := authorityAcc.ID()
			holderID := holderAcc.ID()

			creds, err := m.getCredentials(authorityID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			payload := &crypto.FreezeTokenPayload{
				FreezeAuthority: authorityID.String(),
				TokenHolder:     holderID.String(),
				TokenID:         selectedToken.ID(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, authorityID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.FreezeTokenSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token frozen!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateUnfreezeToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0: // Pick the freeze authority account (signs)
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if len(accounts) < 2 {
				m.statusMsg = "Need at least 2 accounts"
				return m, nil
			}
			m.tokenSourceIndex = 0
			if m.tokenSourceIndex == m.tokenAccountIndex {
				m.tokenSourceIndex = 1
			}
			m.tokenStep = 1
			m.statusMsg = ""
		}

	case 1: // Pick the token holder account
		switch msg.String() {
		case "up", "k":
			if m.tokenSourceIndex > 0 {
				m.tokenSourceIndex--
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex > 0 {
						m.tokenSourceIndex--
					} else {
						m.tokenSourceIndex++
					}
				}
			}
		case "down", "j":
			if m.tokenSourceIndex < len(accounts)-1 {
				m.tokenSourceIndex++
				if m.tokenSourceIndex == m.tokenAccountIndex {
					if m.tokenSourceIndex < len(accounts)-1 {
						m.tokenSourceIndex++
					} else {
						m.tokenSourceIndex--
					}
				}
			}
		case "enter":
			if err := m.refreshTokensForIndex(m.tokenSourceIndex); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			authorityID := accounts[m.tokenAccountIndex].ID()
			var filtered []*scalegraph.Token
			for _, tok := range m.cachedTokens {
				if fa := tok.FreezeAddress(); fa != nil && *fa == authorityID && tok.Frozen() {
					filtered = append(filtered, tok)
				}
			}
			if len(filtered) == 0 {
				m.statusMsg = "No frozen tokens for this authority in selected account"
				return m, nil
			}
			m.cachedTokens = filtered
			m.tokenTokenIndex = 0
			m.tokenStep = 2
			m.statusMsg = ""
		}

	case 2: // Pick the token to unfreeze
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
			}
		case "enter":
			authorityAcc := accounts[m.tokenAccountIndex]
			holderAcc := accounts[m.tokenSourceIndex]
			selectedToken := tokens[m.tokenTokenIndex]
			authorityID := authorityAcc.ID()
			holderID := holderAcc.ID()

			creds, err := m.getCredentials(authorityID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			payload := &crypto.UnfreezeTokenPayload{
				FreezeAuthority: authorityID.String(),
				TokenHolder:     holderID.String(),
				TokenID:         selectedToken.ID(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, authorityID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.UnfreezeTokenSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			_ = m.refreshAccounts()
			m.statusMsg = "Token unfrozen!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateTokenList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc", "q":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.tokenListOffset = 0
			m.tokenTokenIndex = 0
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0:
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			if err := m.refreshTokens(); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.tokenStep = 1
			m.tokenTokenIndex = 0
			m.tokenListOffset = 0
		}
	case 1:
		tokens := m.cachedTokens
		switch msg.String() {
		case "up", "k":
			if m.tokenTokenIndex > 0 {
				m.tokenTokenIndex--
				if m.tokenTokenIndex < m.tokenListOffset {
					m.tokenListOffset = m.tokenTokenIndex
				}
			}
		case "down", "j":
			if m.tokenTokenIndex < len(tokens)-1 {
				m.tokenTokenIndex++
				maxDisplay := 8
				if m.height > 20 {
					maxDisplay = m.height - 16
				}
				if maxDisplay < 3 {
					maxDisplay = 3
				}
				if m.tokenTokenIndex >= m.tokenListOffset+maxDisplay {
					m.tokenListOffset = m.tokenTokenIndex - maxDisplay + 1
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateLookupToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.cachedAccounts

	switch msg.String() {
	case "esc":
		if m.tokenStep > 0 {
			m.tokenStep--
			m.statusMsg = ""
			m.lookupResult = nil
			if m.tokenStep == 1 {
				m.lookupTokenInput.Focus()
			}
		} else {
			m.view = ViewTokenMenu
		}
		return m, nil
	}

	switch m.tokenStep {
	case 0: // Select account
		switch msg.String() {
		case "up", "k":
			if m.tokenAccountIndex > 0 {
				m.tokenAccountIndex--
			}
		case "down", "j":
			if m.tokenAccountIndex < len(accounts)-1 {
				m.tokenAccountIndex++
			}
		case "enter":
			m.tokenStep = 1
			m.lookupTokenInput.SetValue("")
			m.lookupTokenInput.Focus()
			m.lookupResult = nil
			m.statusMsg = ""
		}
	case 1: // Enter token ID
		switch msg.String() {
		case "enter":
			tokenID := m.lookupTokenInput.Value()
			if tokenID == "" {
				m.statusMsg = "Token ID cannot be empty"
				return m, nil
			}

			signerID := accounts[m.tokenAccountIndex].ID()
			creds, err := m.getCredentials(signerID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}

			payload := &crypto.LookupTokenPayload{
				TokenID:   tokenID,
				AccountID: signerID.String(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, signerID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}

			resp, err := m.app.LookupTokenSigned(context.Background(), signedEnvelope)
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}

			m.lookupResult = resp.Token
			m.lookupTokenInput.Blur()
			m.tokenStep = 2
			m.statusMsg = ""
		default:
			var cmd tea.Cmd
			m.lookupTokenInput, cmd = m.lookupTokenInput.Update(msg)
			return m, cmd
		}
	case 2: // Show result
		switch msg.String() {
		case "enter", "q":
			m.view = ViewTokenMenu
			m.tokenStep = 0
			m.lookupResult = nil
		}
	}
	return m, nil
}

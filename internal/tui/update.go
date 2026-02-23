package tui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

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
		case ViewListAccounts:
			return m.updateListAccounts(msg)
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
		case ViewTransferToken:
			return m.updateTransferToken(msg)
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
		case 1: // List Accounts
			m.view = ViewListAccounts
		case 2: // View Account
			m.view = ViewAccountDetail
			m.selectedAccountIndex = 0
		case 3: // Send Money
			m.view = ViewSendMoney
			m.sendStep = 0
			m.sendFromIndex = 0
			m.sendToIndex = 0
			m.sendAmount = ""
		case 4: // View Virtual Nodes
			m.view = ViewVirtualNodes
			m.sendAmount = ""
		case 5: // Token Operations
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

func (m Model) updateListAccounts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.view = ViewMenu
	}
	return m, nil
}

func (m Model) updateAccountDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accountCount, err := m.app.AccountCount(context.Background())
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}

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
			// Get transaction count to set default to latest transaction
			accounts, err := m.app.GetAccounts(context.Background())
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			if m.selectedAccountIndex < len(accounts) {
				blocks := accounts[m.selectedAccountIndex].Blockchain().GetBlocks()
				txCount := 0
				for _, b := range blocks {
					if b.Transaction() != nil {
						txCount++
					}
				}
				// Select the latest transaction (highest index)
				m.selectedTransactionIndex = txCount - 1
				if m.selectedTransactionIndex < 0 {
					m.selectedTransactionIndex = 0
				}
			}
			m.transactionOffset = 0
			m.view = ViewAccountDetailSingle
		}
	}
	return m, nil
}

func (m Model) updateAccountDetailSingle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
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
	accountCount, err := m.app.AccountCount(context.Background())
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}

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
			accounts, err := m.app.GetAccounts(ctx)
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			fromID := accounts[m.sendFromIndex].ID()
			toID := accounts[m.sendToIndex].ID()

			// Get account to calculate nonce
			fromAccount := accounts[m.sendFromIndex]
			nonce := fromAccount.GetNonce() + 1

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

			err = m.app.TransferSigned(ctx, fromID, toID, amount, signedEnvelope)
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}

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
	const tokenMenuItemCount = 3
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
		m.tokenClawbackIndex = 0
		m.tokenTokenIndex = 0
		m.statusMsg = ""
		switch m.tokenMenuCursor {
		case 0:
			m.view = ViewMintToken
			m.tokenValueInput.SetValue("")
			m.tokenValueInput.Blur()
		case 1:
			m.view = ViewAuthorizeTokenTransfer
		case 2:
			m.view = ViewTransferToken
		}
	}
	return m, nil
}

func (m Model) updateMintToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}

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
		// clawback list size: 1 ("No clawback") + len(accounts) - 1 (skip minting account)
		clawbackCount := len(accounts)
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
			// Resolve clawback address (nil if index 0)
			var clawbackAddr *scalegraph.ScalegraphId
			if m.tokenClawbackIndex > 0 {
				idx := 0
				for i, acc := range accounts {
					if i == m.tokenAccountIndex {
						continue
					}
					idx++
					if idx == m.tokenClawbackIndex {
						id := acc.ID()
						clawbackAddr = &id
						break
					}
				}
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

			payload := &crypto.MintTokenPayload{
				TokenValue: m.tokenValueInput.Value(),
			}
			if clawbackAddr != nil {
				clawbackStr := clawbackAddr.String()
				payload.ClawbackAddress = &clawbackStr
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
			m.statusMsg = "Token minted! ID: " + tokenIDPreview
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateAuthorizeTokenTransfer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}

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
			tokens := accounts[m.tokenAccountIndex].GetTokens()
			if len(tokens) == 0 {
				m.statusMsg = "This account has no owned tokens"
				return m, nil
			}
			m.tokenStep = 1
			m.tokenTokenIndex = 0
			m.statusMsg = ""
		}
	case 1:
		tokens := accounts[m.tokenAccountIndex].GetTokens()
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
			accountID := accounts[m.tokenAccountIndex].ID()
			creds, err := m.getCredentials(accountID)
			if err != nil {
				m.statusMsg = "Failed to load credentials: " + err.Error()
				return m, nil
			}
			privKey, err := crypto.DecodePrivateKeyPEM([]byte(creds.PrivateKeyPEM))
			if err != nil {
				m.statusMsg = "Failed to parse private key: " + err.Error()
				return m, nil
			}
			payload := &crypto.AuthorizeTokenTransferPayload{
				AccountID: accountID.String(),
				TokenID:   selectedToken.ID(),
			}
			signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, accountID.String(), creds.CertificatePEM)
			if err != nil {
				m.statusMsg = "Failed to sign request: " + err.Error()
				return m, nil
			}
			if _, err := m.app.AuthorizeTokenTransferSigned(context.Background(), signedEnvelope); err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.statusMsg = "Token authorized for transfer!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

func (m Model) updateTransferToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}

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
			tokens := accounts[m.tokenAccountIndex].GetTokens()
			if len(tokens) == 0 {
				m.statusMsg = "This account has no owned tokens"
				return m, nil
			}
			m.tokenStep = 1
			m.tokenTokenIndex = 0
			m.statusMsg = ""
		}
	case 1:
		tokens := accounts[m.tokenAccountIndex].GetTokens()
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
			tokens := fromAcc.GetTokens()
			selectedToken := tokens[m.tokenTokenIndex]
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
			m.statusMsg = "Token transferred!"
			m.view = ViewTokenMenu
			m.tokenStep = 0
		}
	}
	return m, nil
}

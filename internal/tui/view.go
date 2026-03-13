package tui

import (
	"fmt"
	"strings"

	"sg-emulator/internal/scalegraph"

	"github.com/charmbracelet/lipgloss"
)

// shortID returns the first n characters of id followed by "...", or the
// full id if it is shorter than or equal to n characters.
func shortID(id string, n int) string {
	if len(id) > n {
		return id[:n] + "..."
	}
	return id
}

// renderStatus renders m.statusMsg word-wrapped to the terminal width so that
// long server error messages do not overflow the screen horizontally.
// Strategy: use a plain Width() style to get correct word-wrap line breaks,
// then trim the trailing spaces lipgloss pads onto each line, then apply the
// colour style. This avoids both truncation (MaxWidth) and fixed-width padding
// (Width) that would break JoinVertical centering.
func (m Model) renderStatus() string {
	if m.statusMsg == "" {
		return ""
	}
	// Leave a small margin on each side so the text doesn't touch the edges.
	maxWidth := m.width - 4
	if maxWidth < 20 {
		maxWidth = 20
	}
	wrapped := lipgloss.NewStyle().Width(maxWidth).Render(m.statusMsg)
	lines := strings.Split(wrapped, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return statusStyle.Render(strings.Join(lines, "\n"))
}

var menuItems = []string{
	"Create Account",
	"View Account",
	"Send Money",
	"View Virtual Nodes",
	"Token Operations",
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var content string

	switch m.view {
	case ViewMenu:
		content = m.viewMenu()
	case ViewCreateAccount:
		content = m.viewCreateAccount()
	case ViewAccountDetail:
		content = m.viewAccountDetail()
	case ViewAccountDetailSingle:
		content = m.viewAccountDetailSingle()
	case ViewTransactionDetail:
		content = m.viewTransactionDetail()
	case ViewSendMoney:
		content = m.viewSendMoney()
	case ViewVirtualNodes:
		content = m.viewVirtualNodes()
	case ViewTokenMenu:
		content = m.viewTokenMenu()
	case ViewMintToken:
		content = m.viewMintToken()
	case ViewAuthorizeTokenTransfer:
		content = m.viewAuthorizeTokenTransfer()
	case ViewUnauthorizeTokenTransfer:
		content = m.viewUnauthorizeTokenTransfer()
	case ViewTransferToken:
		content = m.viewTransferToken()
	case ViewTokenList:
		content = m.viewTokenList()
	case ViewBurnToken:
		content = m.viewBurnToken()
	case ViewClawbackToken:
		content = m.viewClawbackToken()
	case ViewLookupToken:
		content = m.viewLookupToken()
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

func (m Model) viewMenu() string {
	title := titleStyle.Render("SG Emulator")

	var menuContent string
	for i, item := range menuItems {
		cursor := "  "
		if i == m.menuCursor {
			cursor = "> "
			item = selectedStyle.Render(item)
		}
		menuContent += cursor + item + "\n"
	}

	help := helpStyle.Render("↑/↓: navigate • enter: select • q: quit")

	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		menuContent,
		"",
		status,
		help,
	)
}

func (m Model) viewCreateAccount() string {
	title := titleStyle.Render("Create Account")

	var inputs string

	// Balance input
	balanceLabel := "Initial Balance:"
	if m.createAccountFocus == 0 {
		balanceLabel = focusedLabelStyle.Render(balanceLabel)
	} else {
		balanceLabel = textLabelStyle.Render(balanceLabel)
	}
	inputs += balanceLabel + "\n" + m.balanceInput.View() + "\n\n"

	// Name input
	nameLabel := "Account Name (optional):"
	if m.createAccountFocus == 1 {
		nameLabel = focusedLabelStyle.Render(nameLabel)
	} else {
		nameLabel = textLabelStyle.Render(nameLabel)
	}
	inputs += nameLabel + "\n" + m.nameInput.View() + "\n\n"

	// Submit button
	submitButton := "[ Submit ]"
	if m.createAccountFocus == 2 {
		submitButton = focusedButtonStyle.Render(submitButton)
	} else {
		submitButton = textLabelStyle.Render(submitButton)
	}
	inputs += submitButton

	help := helpStyle.Render("tab/shift+tab: navigate • enter: submit/next • esc: back")

	statusRaw := m.renderStatus()
	status := ""
	if statusRaw != "" {
		status = statusRaw + "\n"
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		inputs,
		"",
		status,
		help,
	)
}

func (m Model) viewAccountDetail() string {
	title := titleStyle.Render("Account Details")
	accounts := m.cachedAccounts

	var content string
	if len(accounts) == 0 {
		content = "No accounts available."
	} else {
		var listContent string
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f",
				m.getAccountDisplayName(acc),
				acc.Balance())
			if i == m.selectedAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			listContent += cursor + line + "\n"
		}
		content = listContent
	}

	help := helpStyle.Render("↑/↓: navigate • enter: view details • esc: back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		help,
	)
}

func (m Model) viewAccountDetailSingle() string {
	title := titleStyle.Render("Account Details")
	accounts := m.cachedAccounts

	if len(accounts) == 0 || m.selectedAccountIndex >= len(accounts) {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"No account selected.",
			"",
			helpStyle.Render("esc: back"),
		)
	}

	selectedAcc := accounts[m.selectedAccountIndex]

	// Display selected account details - responsive width
	boxWidth := 60
	if m.width > 0 {
		boxWidth = m.width - 10
		if boxWidth > 80 {
			boxWidth = 80
		}
		if boxWidth < 40 {
			boxWidth = 40
		}
	}
	details := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(boxWidth)

	var detailContent string

	// Name
	name := m.getAccountDisplayName(selectedAcc)
	detailContent += fmt.Sprintf("Name:    %s\n", lipgloss.NewStyle().Bold(true).Render(name))

	// ID
	detailContent += fmt.Sprintf("ID:      %s\n", selectedAcc.ID().String())

	// Balance
	detailContent += fmt.Sprintf("Balance: %.2f\n", selectedAcc.Balance())

	// MBR
	detailContent += fmt.Sprintf("MBR:     %.2f\n", selectedAcc.MBR())

	// Transaction History
	blockchain := selectedAcc.Blockchain()
	blocks := blockchain.GetBlocks()

	// Collect transactions (skip genesis block)
	var transactions []scalegraph.ITransaction
	for _, block := range blocks {
		if block.Transaction() != nil {
			transactions = append(transactions, block.Transaction())
		}
	}

	detailContent += "\n" + lipgloss.NewStyle().Bold(true).Render("Transaction History:") + "\n"

	if len(transactions) == 0 {
		detailContent += "  No transactions yet\n"
	} else {
		// Calculate max transactions to display based on terminal height
		maxDisplay := 5
		if m.height > 20 {
			maxDisplay = m.height - 15
		}
		if maxDisplay < 3 {
			maxDisplay = 3
		}
		if maxDisplay > 15 {
			maxDisplay = 15
		}

		startIdx := len(transactions) - 1 - m.transactionOffset
		endIdx := startIdx - maxDisplay + 1
		if endIdx < 0 {
			endIdx = 0
		}

		// Show transactions from newest to oldest
		for i := startIdx; i >= endIdx; i-- {
			tx := transactions[i]

			// Cursor for selection
			cursor := "  "
			if i == m.selectedTransactionIndex {
				cursor = "> "
			}

			// Arrow indicator for scroll position
			arrow := " "
			if i == startIdx && m.transactionOffset > 0 {
				arrow = "↓"
			} else if i == endIdx && endIdx > 0 {
				arrow = "↑"
			}

			// Transaction number (1-indexed, oldest=1)
			txNum := i + 1
			line := fmt.Sprintf("%s#%d: ", arrow, txNum)

			switch tx.Type() {
			case scalegraph.Transfer:
				if transferTx, ok := tx.(*scalegraph.TransferTransaction); ok {
					if tx.Sender() != nil && tx.Sender().ID() == selectedAcc.ID() { // Sent transaction
						line += fmt.Sprintf("Send -%.2f to %s", transferTx.Amount(), m.getAccountDisplayName(tx.Receiver()))
					} else if tx.Receiver() != nil && tx.Receiver().ID() == selectedAcc.ID() { // Received transaction
						line += fmt.Sprintf("Receive +%.2f from %s", transferTx.Amount(), m.getAccountDisplayName(tx.Sender()))
					}
				}
			case scalegraph.Mint:
				if mintTx, ok := tx.(*scalegraph.MintTransaction); ok {
					line += fmt.Sprintf("Mint +%.2f", mintTx.Amount())
				}
			case scalegraph.Burn:
				if burnTx, ok := tx.(*scalegraph.BurnTransaction); ok {
					line += fmt.Sprintf("Burn -%.2f", burnTx.Amount())
				}
			case scalegraph.MintToken:
				if mintTokTx, ok := tx.(*scalegraph.MintTokenTransaction); ok {
					line += fmt.Sprintf("Token Mint  val:%-10s id:%s", mintTokTx.Token().Value(), shortID(mintTokTx.Token().ID(), 8))
				}
			case scalegraph.TransferToken:
				if xferTokTx, ok := tx.(*scalegraph.TransferTokenTransaction); ok {
					tokID := shortID(xferTokTx.Token().ID(), 8)
					if tx.Sender() != nil && tx.Sender().ID() == selectedAcc.ID() {
						line += fmt.Sprintf("Token Send  val:%-10s id:%s to %s", xferTokTx.Token().Value(), tokID, m.getAccountDisplayName(tx.Receiver()))
					} else {
						line += fmt.Sprintf("Token Recv  val:%-10s id:%s from %s", xferTokTx.Token().Value(), tokID, m.getAccountDisplayName(tx.Sender()))
					}
				}
			case scalegraph.AuthorizeTokenTransfer:
				if authTx, ok := tx.(*scalegraph.AuthorizeTokenTransferTransaction); ok {
					tokID := "(unknown)"
					if authTx.TokenId() != nil {
						tokID = shortID(*authTx.TokenId(), 8)
					}
					line += fmt.Sprintf("Token Auth  id:%s", tokID)
				}
			case scalegraph.UnauthorizeTokenTransfer:
				if unauthTx, ok := tx.(*scalegraph.UnauthorizeTokenTransferTransaction); ok {
					tokID := "(unknown)"
					if unauthTx.TokenId() != nil {
						tokID = shortID(*unauthTx.TokenId(), 8)
					}
					line += fmt.Sprintf("Token Unauth id:%s", tokID)
				}
			case scalegraph.BurnToken:
				if burnTokTx, ok := tx.(*scalegraph.BurnTokenTransaction); ok {
					line += fmt.Sprintf("Token Burn  id:%s", shortID(burnTokTx.TokenID(), 8))
				}
			case scalegraph.ClawbackTokenTransfer:
				if clawTx, ok := tx.(*scalegraph.ClawbackTokenTransaction); ok {
					tok := clawTx.Token()
					if tx.Sender() != nil && tx.Sender().ID() == selectedAcc.ID() {
						line += fmt.Sprintf("Token Claw  val:%-10s id:%s (out)", tok.Value(), shortID(tok.ID(), 8))
					} else {
						line += fmt.Sprintf("Token Claw  val:%-10s id:%s (in)", tok.Value(), shortID(tok.ID(), 8))
					}
				}
			}

			if i == m.selectedTransactionIndex {
				line = selectedStyle.Render(line)
			}
			detailContent += cursor + line + "\n"
		}

		// Show navigation hint
		if len(transactions) > maxDisplay {
			detailContent += fmt.Sprintf("\n  Showing %d-%d of %d transactions\n",
				endIdx+1, startIdx+1, len(transactions))
		}
	}

	content := details.Render(detailContent)

	help := helpStyle.Render("↑/↓: navigate • enter: view details • esc: back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		help,
	)
}

func (m Model) viewTransactionDetail() string {
	title := titleStyle.Render("Transaction Details")
	accounts := m.cachedAccounts

	if len(accounts) == 0 || m.selectedAccountIndex >= len(accounts) {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"No account selected.",
			"",
			helpStyle.Render("esc: back"),
		)
	}

	selectedAcc := accounts[m.selectedAccountIndex]
	blockchain := selectedAcc.Blockchain()
	blocks := blockchain.GetBlocks()

	// Collect transactions (skip genesis block)
	var transactions []scalegraph.ITransaction
	for _, block := range blocks {
		if block.Transaction() != nil {
			transactions = append(transactions, block.Transaction())
		}
	}

	if len(transactions) == 0 || m.selectedTransactionIndex >= len(transactions) {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"No transaction selected.",
			"",
			helpStyle.Render("esc: back"),
		)
	}

	tx := transactions[m.selectedTransactionIndex]

	// Display transaction details - responsive width
	boxWidth := 70
	if m.width > 0 {
		boxWidth = m.width - 10
		if boxWidth > 90 {
			boxWidth = 90
		}
		if boxWidth < 50 {
			boxWidth = 50
		}
	}
	details := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(boxWidth)

	var detailContent string

	// Transaction ID
	detailContent += lipgloss.NewStyle().Bold(true).Render("Transaction ID:") + "\n"
	detailContent += fmt.Sprintf("  %s\n\n", tx.ID().String())

	// Transaction Number
	detailContent += lipgloss.NewStyle().Bold(true).Render("Transaction #:") + "\n"
	detailContent += fmt.Sprintf("  %d (of %d)\n\n", m.selectedTransactionIndex+1, len(transactions))

	// Type and Details
	detailContent += lipgloss.NewStyle().Bold(true).Render("Type:") + "\n"

	isTokenTx := false
	var amount float64
	switch tx.Type() {
	case scalegraph.Mint:
		if mintTx, ok := tx.(*scalegraph.MintTransaction); ok {
			amount = mintTx.Amount()
		}
		// Mint transaction
		detailContent += "  Mint (Initial Balance)\n\n"
		detailContent += lipgloss.NewStyle().Bold(true).Render("To:") + "\n"
		detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(selectedAcc))
		detailContent += fmt.Sprintf("  %s\n\n", selectedAcc.ID().String())
	case scalegraph.Transfer:
		if transferTx, ok := tx.(*scalegraph.TransferTransaction); ok {
			amount = transferTx.Amount()
		}
		if tx.Sender() != nil && tx.Sender().ID() == selectedAcc.ID() {
			// Sent transaction
			detailContent += "  Send\n\n"
			detailContent += lipgloss.NewStyle().Bold(true).Render("From:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(tx.Sender()))
			detailContent += fmt.Sprintf("  %s\n\n", tx.Sender().ID().String())
			detailContent += lipgloss.NewStyle().Bold(true).Render("To:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(tx.Receiver()))
			detailContent += fmt.Sprintf("  %s\n\n", tx.Receiver().ID().String())
		} else {
			// Received transaction
			detailContent += "  Receive\n\n"
			detailContent += lipgloss.NewStyle().Bold(true).Render("From:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(tx.Sender()))
			detailContent += fmt.Sprintf("  %s\n\n", tx.Sender().ID().String())
			detailContent += lipgloss.NewStyle().Bold(true).Render("To:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(tx.Receiver()))
			detailContent += fmt.Sprintf("  %s\n\n", tx.Receiver().ID().String())
		}
	case scalegraph.Burn:
		if burnTx, ok := tx.(*scalegraph.BurnTransaction); ok {
			amount = burnTx.Amount()
		}
		detailContent += "  Burn\n\n"
		detailContent += lipgloss.NewStyle().Bold(true).Render("From:") + "\n"
		detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(tx.Receiver()))
		detailContent += fmt.Sprintf("  %s\n\n", tx.Receiver().ID().String())
	case scalegraph.MintToken:
		isTokenTx = true
		if mintTokTx, ok := tx.(*scalegraph.MintTokenTransaction); ok {
			detailContent += "  Mint Token\n\n"
			detailContent += lipgloss.NewStyle().Bold(true).Render("To:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(mintTokTx.Receiver()))
			detailContent += fmt.Sprintf("  %s\n\n", mintTokTx.Receiver().ID().String())
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token Value:") + "\n"
			detailContent += fmt.Sprintf("  %s\n\n", mintTokTx.Token().Value())
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token ID:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", shortID(mintTokTx.Token().ID(), 16))
		}
	case scalegraph.TransferToken:
		isTokenTx = true
		if xferTokTx, ok := tx.(*scalegraph.TransferTokenTransaction); ok {
			if tx.Sender() != nil && tx.Sender().ID() == selectedAcc.ID() {
				detailContent += "  Transfer Token (Sent)\n\n"
			} else {
				detailContent += "  Transfer Token (Received)\n\n"
			}
			detailContent += lipgloss.NewStyle().Bold(true).Render("From:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(xferTokTx.Sender()))
			detailContent += fmt.Sprintf("  %s\n\n", xferTokTx.Sender().ID().String())
			detailContent += lipgloss.NewStyle().Bold(true).Render("To:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(xferTokTx.Receiver()))
			detailContent += fmt.Sprintf("  %s\n\n", xferTokTx.Receiver().ID().String())
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token Value:") + "\n"
			detailContent += fmt.Sprintf("  %s\n\n", xferTokTx.Token().Value())
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token ID:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", shortID(xferTokTx.Token().ID(), 16))
		}
	case scalegraph.AuthorizeTokenTransfer:
		isTokenTx = true
		if authTx, ok := tx.(*scalegraph.AuthorizeTokenTransferTransaction); ok {
			detailContent += "  Authorize Token Transfer\n\n"
			detailContent += lipgloss.NewStyle().Bold(true).Render("Account:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(authTx.Sender()))
			detailContent += fmt.Sprintf("  %s\n\n", authTx.Sender().ID().String())
			tokID := "(unknown)"
			if authTx.TokenId() != nil {
				tokID = shortID(*authTx.TokenId(), 16)
			}
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token ID:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", tokID)
		}
	case scalegraph.UnauthorizeTokenTransfer:
		isTokenTx = true
		if unauthTx, ok := tx.(*scalegraph.UnauthorizeTokenTransferTransaction); ok {
			detailContent += "  Unauthorize Token Transfer\n\n"
			detailContent += lipgloss.NewStyle().Bold(true).Render("Account:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(unauthTx.Sender()))
			detailContent += fmt.Sprintf("  %s\n\n", unauthTx.Sender().ID().String())
			tokID := "(unknown)"
			if unauthTx.TokenId() != nil {
				tokID = shortID(*unauthTx.TokenId(), 16)
			}
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token ID:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", tokID)
		}
	case scalegraph.BurnToken:
		isTokenTx = true
		if burnTokTx, ok := tx.(*scalegraph.BurnTokenTransaction); ok {
			detailContent += "  Burn Token\n\n"
			detailContent += lipgloss.NewStyle().Bold(true).Render("Account:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(burnTokTx.Sender()))
			detailContent += fmt.Sprintf("  %s\n\n", burnTokTx.Sender().ID().String())
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token ID:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", shortID(burnTokTx.TokenID(), 16))
		}
	case scalegraph.ClawbackTokenTransfer:
		isTokenTx = true
		if clawTx, ok := tx.(*scalegraph.ClawbackTokenTransaction); ok {
			detailContent += "  Clawback Token\n\n"
			detailContent += lipgloss.NewStyle().Bold(true).Render("From (holder):") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(clawTx.Sender()))
			detailContent += fmt.Sprintf("  %s\n\n", clawTx.Sender().ID().String())
			detailContent += lipgloss.NewStyle().Bold(true).Render("To (authority):") + "\n"
			detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(clawTx.Receiver()))
			detailContent += fmt.Sprintf("  %s\n\n", clawTx.Receiver().ID().String())
			tok := clawTx.Token()
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token Value:") + "\n"
			detailContent += fmt.Sprintf("  %s\n\n", tok.Value())
			detailContent += lipgloss.NewStyle().Bold(true).Render("Token ID:") + "\n"
			detailContent += fmt.Sprintf("  %s\n", shortID(tok.ID(), 16))
		}
	}

	// Amount (only for balance-changing transactions)
	if !isTokenTx {
		detailContent += lipgloss.NewStyle().Bold(true).Render("Amount:") + "\n"
		var amountStr string
		if tx.Sender() == nil || (tx.Sender() != nil && tx.Sender().ID() != selectedAcc.ID()) {
			amountStr = "  +" + fmt.Sprintf("%.2f", amount)
		} else {
			amountStr = "  -" + fmt.Sprintf("%.2f", amount)
		}
		detailContent += amountStr + "\n"
	}

	content := details.Render(detailContent)

	help := helpStyle.Render("esc: back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		help,
	)
}

func (m Model) viewSendMoney() string {
	title := titleStyle.Render("Send Money")
	accounts := m.cachedAccounts

	if len(accounts) < 2 {
		content := "Need at least 2 accounts to send money."
		help := helpStyle.Render("esc: back")
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
			"",
			help,
		)
	}

	var content string

	switch m.sendStep {
	case 0: // Select from account
		content = "Select FROM account:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f",
				m.getAccountDisplayName(acc),
				acc.Balance())
			if i == m.sendFromIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	case 1: // Select to account
		content = fmt.Sprintf("From: %s\n\n", m.getAccountDisplayName(accounts[m.sendFromIndex]))
		content += "Select TO account:\n\n"
		for i, acc := range accounts {
			if i == m.sendFromIndex {
				continue
			}
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f",
				m.getAccountDisplayName(acc),
				acc.Balance())
			if i == m.sendToIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	case 2: // Enter amount
		content = fmt.Sprintf("From: %s\n", m.getAccountDisplayName(accounts[m.sendFromIndex]))
		content += fmt.Sprintf("To: %s\n\n", m.getAccountDisplayName(accounts[m.sendToIndex]))
		content += "Enter amount: " + m.sendAmount + "█"
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")

	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		status,
		help,
	)
}

func (m Model) viewVirtualNodes() string {
	title := titleStyle.Render("Virtual Nodes")

	var content string

	if m.server == nil {
		content = "Server not available"
	} else {
		registry := m.server.Registry()
		vapps := registry.List()

		if len(vapps) == 0 {
			content = "No virtual nodes created.\n\n"
			content += helpStyle.Render("Use -rest N or -grpc N flags to create virtual nodes")
		} else {
			content = fmt.Sprintf("Total Virtual Nodes: %d\n\n", len(vapps))

			for i, vapp := range vapps {
				content += fmt.Sprintf("Node %d:\n", i+1)
				content += fmt.Sprintf("  ID:        %s\n", vapp.ID().String())

				addresses := vapp.Addresses()
				if len(addresses) == 0 {
					content += "  Transport: none\n"
				} else {
					for transportType, addr := range addresses {
						content += fmt.Sprintf("  Transport: %s\n", transportType)
						content += fmt.Sprintf("  Address:   %s\n", addr)
					}
				}
				content += "\n"
			}
		}
	}

	help := helpStyle.Render("esc: back to menu")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		help,
	)
}

func (m Model) viewTokenMenu() string {
	title := titleStyle.Render("Token Operations")

	tokenMenuItems := []string{
		"Mint Token",
		"Authorize Token Transfer",
		"Unauthorize Token Transfer",
		"Transfer Token",
		"View Account Tokens",
		"Burn Token",
		"Clawback Token",
		"Lookup Token",
	}

	var content string
	for i, item := range tokenMenuItems {
		cursor := "  "
		if i == m.tokenMenuCursor {
			cursor = "> "
			item = selectedStyle.Render(item)
		}
		content += cursor + item + "\n"
	}

	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		help,
	)
}

func (m Model) viewMintToken() string {
	title := titleStyle.Render("Mint Token")

	accounts := m.cachedAccounts
	if len(accounts) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "No accounts available. Create an account first.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0:
		content = "Select account to mint token for:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	case 1:
		content = fmt.Sprintf("Account: %s\n\n", m.getAccountDisplayName(accounts[m.tokenAccountIndex]))
		content += "Token value:\n"
		content += m.tokenValueInput.View()
	case 2:
		content = fmt.Sprintf("Account: %s\n", m.getAccountDisplayName(accounts[m.tokenAccountIndex]))
		content += fmt.Sprintf("Token value: %s\n\n", m.tokenValueInput.Value())
		content += "Select clawback address (or none):\n\n"
		// Index 0 = No clawback; index 1..N = other accounts
		noClawbackCursor := "  "
		noClawbackLabel := "No clawback"
		if m.tokenClawbackIndex == 0 {
			noClawbackCursor = "> "
			noClawbackLabel = selectedStyle.Render(noClawbackLabel)
		}
		content += noClawbackCursor + noClawbackLabel + "\n"
		for i, acc := range accounts {
			clawbackIdx := i + 1
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenAccountIndex {
				line += " (minter)"
			}
			if clawbackIdx == m.tokenClawbackIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")

	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		status,
		help,
	)
}

func (m Model) viewAuthorizeTokenTransfer() string {
	title := titleStyle.Render("Authorize Token Transfer")

	accounts := m.cachedAccounts
	if len(accounts) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "No accounts available.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0: // Select receiving account
		content = "Step 1/3 — Select receiving account:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 1: // Select source account (token owner)
		receiver := accounts[m.tokenAccountIndex]
		content = fmt.Sprintf("Receiver: %s\n\n", m.getAccountDisplayName(receiver))
		content += "Step 2/3 — Select source account (token owner):\n\n"
		for i, acc := range accounts {
			if i == m.tokenAccountIndex {
				continue // skip the receiver
			}
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f  Tokens: %d",
				m.getAccountDisplayName(acc), acc.Balance(), len(acc.GetTokens()))
			if i == m.tokenSourceIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 2: // Select token from source account
		receiver := accounts[m.tokenAccountIndex]
		source := accounts[m.tokenSourceIndex]
		content = fmt.Sprintf("Receiver: %s\n", m.getAccountDisplayName(receiver))
		content += fmt.Sprintf("Source:   %s\n\n", m.getAccountDisplayName(source))
		tokens := m.cachedTokens
		if len(tokens) == 0 {
			content += "No owned tokens found."
		} else {
			content += "Step 3/3 — Select token to authorize receiving:\n\n"
			for i, tok := range tokens {
				cursor := "  "
				line := fmt.Sprintf("Value: %-12s  ID: %s", tok.Value(), shortID(tok.ID(), 8))
				if i == m.tokenTokenIndex {
					cursor = "> "
					line = selectedStyle.Render(line)
				}
				content += cursor + line + "\n"
			}
		}
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")

	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		status,
		help,
	)
}

func (m Model) viewUnauthorizeTokenTransfer() string {
	title := titleStyle.Render("Unauthorize Token Transfer")

	accounts := m.cachedAccounts
	if len(accounts) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "No accounts available.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0: // Select the account revoking authorization (receiver)
		content = "Step 1/3 — Select account revoking authorization:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 1: // Select the source account (token owner) to browse tokens from
		receiver := accounts[m.tokenAccountIndex]
		content = fmt.Sprintf("Receiver: %s\n\n", m.getAccountDisplayName(receiver))
		content += "Step 2/3 — Select source account (token owner):\n\n"
		for i, acc := range accounts {
			if i == m.tokenAccountIndex {
				continue
			}
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f  Tokens: %d",
				m.getAccountDisplayName(acc), acc.Balance(), len(acc.GetTokens()))
			if i == m.tokenSourceIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 2: // Select token from source account
		receiver := accounts[m.tokenAccountIndex]
		source := accounts[m.tokenSourceIndex]
		content = fmt.Sprintf("Receiver: %s\n", m.getAccountDisplayName(receiver))
		content += fmt.Sprintf("Source:   %s\n\n", m.getAccountDisplayName(source))
		tokens := m.cachedTokens
		if len(tokens) == 0 {
			content += "No owned tokens found."
		} else {
			content += "Step 3/3 — Select token to unauthorize:\n\n"
			for i, tok := range tokens {
				cursor := "  "
				line := fmt.Sprintf("Value: %-12s  ID: %s", tok.Value(), shortID(tok.ID(), 8))
				if i == m.tokenTokenIndex {
					cursor = "> "
					line = selectedStyle.Render(line)
				}
				content += cursor + line + "\n"
			}
		}
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")

	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		status,
		help,
	)
}

func (m Model) viewTransferToken() string {
	title := titleStyle.Render("Transfer Token")

	accounts := m.cachedAccounts
	if len(accounts) < 2 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "Need at least 2 accounts to transfer a token.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0:
		content = "Select FROM account:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	case 1:
		acc := accounts[m.tokenAccountIndex]
		content = fmt.Sprintf("From: %s\n\n", m.getAccountDisplayName(acc))
		tokens := m.cachedTokens
		if len(tokens) == 0 {
			content += "No owned tokens found."
		} else {
			content += "Select token to transfer:\n\n"
			for i, tok := range tokens {
				cursor := "  "
				line := fmt.Sprintf("Value: %-12s  ID: %s", tok.Value(), shortID(tok.ID(), 8))
				if i == m.tokenTokenIndex {
					cursor = "> "
					line = selectedStyle.Render(line)
				}
				content += cursor + line + "\n"
			}
		}
	case 2:
		fromAcc := accounts[m.tokenAccountIndex]
		tokens := m.cachedTokens
		var tokenLabel string
		if len(tokens) > m.tokenTokenIndex {
			tok := tokens[m.tokenTokenIndex]
			tokenLabel = fmt.Sprintf("Value: %s  ID: %s", tok.Value(), shortID(tok.ID(), 8))
		}
		content = fmt.Sprintf("From: %s\n", m.getAccountDisplayName(fromAcc))
		content += fmt.Sprintf("Token: %s\n\n", tokenLabel)
		content += "Select TO account:\n\n"
		for i, acc := range accounts {
			if i == m.tokenAccountIndex {
				continue
			}
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenToAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")

	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		status,
		help,
	)
}

func (m Model) viewBurnToken() string {
	title := titleStyle.Render("Burn Token")

	accounts := m.cachedAccounts
	if len(accounts) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "No accounts available.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0: // Select account
		content = "Select account that owns the token to burn:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f  Tokens: %d",
				m.getAccountDisplayName(acc), acc.Balance(), len(acc.GetTokens()))
			if i == m.tokenAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 1: // Select token
		acc := accounts[m.tokenAccountIndex]
		content = fmt.Sprintf("Account: %s\n\n", m.getAccountDisplayName(acc))
		tokens := m.cachedTokens
		if len(tokens) == 0 {
			content += "No owned tokens found."
		} else {
			content += "Select token to burn:\n\n"
			for i, tok := range tokens {
				cursor := "  "
				line := fmt.Sprintf("Value: %-12s  ID: %s", tok.Value(), shortID(tok.ID(), 8))
				if i == m.tokenTokenIndex {
					cursor = "> "
					line = selectedStyle.Render(line)
				}
				content += cursor + line + "\n"
			}
		}
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")
	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		status,
		help,
	)
}

func (m Model) viewClawbackToken() string {
	title := titleStyle.Render("Clawback Token")

	accounts := m.cachedAccounts
	if len(accounts) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "No accounts available.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0: // Select clawback authority account (To — signs the request)
		content = "Step 1/3 — Select clawback authority account:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 1: // Select token holder account (From)
		authority := accounts[m.tokenAccountIndex]
		content = fmt.Sprintf("Authority: %s\n\n", m.getAccountDisplayName(authority))
		content += "Step 2/3 — Select token holder account:\n\n"
		for i, acc := range accounts {
			if i == m.tokenAccountIndex {
				continue
			}
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f  Tokens: %d",
				m.getAccountDisplayName(acc), acc.Balance(), len(acc.GetTokens()))
			if i == m.tokenSourceIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 2: // Select token to claw back (filtered to this authority)
		authority := accounts[m.tokenAccountIndex]
		holder := accounts[m.tokenSourceIndex]
		content = fmt.Sprintf("Authority: %s\n", m.getAccountDisplayName(authority))
		content += fmt.Sprintf("Holder:    %s\n\n", m.getAccountDisplayName(holder))
		tokens := m.cachedTokens
		if len(tokens) == 0 {
			content += "No clawable tokens found."
		} else {
			content += "Step 3/3 — Select token to claw back:\n\n"
			for i, tok := range tokens {
				cursor := "  "
				line := fmt.Sprintf("Value: %-12s  ID: %s", tok.Value(), shortID(tok.ID(), 8))
				if i == m.tokenTokenIndex {
					cursor = "> "
					line = selectedStyle.Render(line)
				}
				content += cursor + line + "\n"
			}
		}
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")
	status := m.renderStatus()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		status,
		help,
	)
}

func (m Model) viewTokenList() string {
	title := titleStyle.Render("Account Tokens")

	accounts := m.cachedAccounts
	if len(accounts) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "No accounts available.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0:
		content = "Select account to inspect:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f", m.getAccountDisplayName(acc), acc.Balance())
			if i == m.tokenAccountIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}

	case 1:
		acc := accounts[m.tokenAccountIndex]
		tokens := m.cachedTokens

		boxWidth := 70
		if m.width > 0 {
			boxWidth = m.width - 10
			if boxWidth > 90 {
				boxWidth = 90
			}
			if boxWidth < 40 {
				boxWidth = 40
			}
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Width(boxWidth)

		var boxContent string
		boxContent += fmt.Sprintf("Account: %s\n", lipgloss.NewStyle().Bold(true).Render(m.getAccountDisplayName(acc)))
		boxContent += fmt.Sprintf("ID:      %s\n\n", acc.ID().String())
		boxContent += lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Tokens (%d):", len(tokens))) + "\n"

		if len(tokens) == 0 {
			boxContent += "  No tokens owned.\n"
		} else {
			// Calculate max rows to display
			maxDisplay := 8
			if m.height > 20 {
				maxDisplay = m.height - 16
			}
			if maxDisplay < 3 {
				maxDisplay = 3
			}
			if maxDisplay > 20 {
				maxDisplay = 20
			}

			startIdx := m.tokenListOffset
			endIdx := startIdx + maxDisplay
			if endIdx > len(tokens) {
				endIdx = len(tokens)
			}

			for i := startIdx; i < endIdx; i++ {
				tok := tokens[i]
				cursor := "  "
				tokID := shortID(tok.ID(), 16)
				clawback := "none"
				if tok.ClawbackAddress() != nil {
					ca := tok.ClawbackAddress()
					caStr := ca.String()
					if len(caStr) > 8 {
						caStr = caStr[:8] + "..."
					}
					clawback = caStr
				}
				line := fmt.Sprintf("#%d  val:%-12s  id:%s  clawback:%s", i+1, tok.Value(), tokID, clawback)
				if i == m.tokenTokenIndex {
					cursor = "> "
					line = selectedStyle.Render(line)
				}
				boxContent += cursor + line + "\n"
			}

			if len(tokens) > maxDisplay {
				boxContent += fmt.Sprintf("\n  Showing %d-%d of %d tokens\n", startIdx+1, endIdx, len(tokens))
			}
		}

		content = box.Render(boxContent)
	}

	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		help,
	)
}

func (m Model) viewLookupToken() string {
	title := titleStyle.Render("Lookup Token")

	accounts := m.cachedAccounts
	if len(accounts) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "", "No accounts available. Create an account first.", "", helpStyle.Render("esc: back"))
	}

	var content string

	switch m.tokenStep {
	case 0:
		content = "Select account to look up token in:\n\n"
		for i, acc := range accounts {
			cursor := "  "
			if i == m.tokenAccountIndex {
				cursor = "> "
				content += cursor + selectedStyle.Render(m.getAccountDisplayName(acc)) + "\n"
			} else {
				content += cursor + m.getAccountDisplayName(acc) + "\n"
			}
		}
	case 1:
		content = fmt.Sprintf("Account: %s\n\n", m.getAccountDisplayName(accounts[m.tokenAccountIndex]))
		content += "Enter token ID:\n"
		content += m.lookupTokenInput.View() + "\n"
	case 2:
		content = fmt.Sprintf("Account: %s\n\n", m.getAccountDisplayName(accounts[m.tokenAccountIndex]))
		if m.lookupResult == nil {
			content += "Token not found.\n"
		} else {
			t := m.lookupResult
			content += "Token found:\n\n"
			content += fmt.Sprintf("  ID:    %s\n", t.ID())
			content += fmt.Sprintf("  Value: %s\n", t.Value())
			if t.ClawbackAddress() != nil {
				content += fmt.Sprintf("  Clawback: %s\n", t.ClawbackAddress().String())
			}
		}
	}

	var help string
	switch m.tokenStep {
	case 0:
		help = helpStyle.Render("↑/↓: navigate • enter: select • esc: back")
	case 1:
		help = helpStyle.Render("enter: lookup • esc: back")
	case 2:
		help = helpStyle.Render("enter/q: back to menu • esc: back")
	}

	parts := []string{title, "", content}
	if status := m.renderStatus(); status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, "", help)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

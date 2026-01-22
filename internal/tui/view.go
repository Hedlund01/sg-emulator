package tui

import (
	"context"
	"fmt"

	"sg-emulator/internal/scalegraph"

	"github.com/charmbracelet/lipgloss"
)

var menuItems = []string{
	"Create Account",
	"List Accounts",
	"View Account",
	"Send Money",
	"View Virtual Nodes",
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
	case ViewListAccounts:
		content = m.viewListAccounts()
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

	status := ""
	if m.statusMsg != "" {
		status = statusStyle.Render(m.statusMsg)
	}

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

	status := ""
	if m.statusMsg != "" {
		status = statusStyle.Render(m.statusMsg) + "\n"
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

func (m Model) viewListAccounts() string {
	title := titleStyle.Render("All Accounts")
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"Error: "+err.Error(),
			"",
			helpStyle.Render("esc: back"),
		)
	}

	var listContent string
	if len(accounts) == 0 {
		listContent = "No accounts yet."
	} else {
		for i, acc := range accounts {
			line := fmt.Sprintf("%d. %s  Balance: %.2f",
				i+1,
				m.getAccountDisplayName(acc),
				acc.Balance())
			listContent += line + "\n"
		}
	}

	help := helpStyle.Render("esc: back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		listContent,
		"",
		help,
	)
}

func (m Model) viewAccountDetail() string {
	title := titleStyle.Render("Account Details")
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"Error: "+err.Error(),
			"",
			helpStyle.Render("esc: back"),
		)
	}

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
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"Error: "+err.Error(),
			"",
			helpStyle.Render("esc: back"),
		)
	}

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

	// Transaction History
	blockchain := selectedAcc.Blockchain()
	blocks := blockchain.GetBlocks()

	// Collect transactions (skip genesis block)
	var transactions []*scalegraph.Transaction
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

			// Determine transaction type and format
			if tx.Sender() == nil {
				// Mint transaction
				line += fmt.Sprintf("Mint +%.2f", tx.Amount())
			} else if tx.Sender().ID() == selectedAcc.ID() {
				// Sent transaction
				toName := m.getAccountDisplayName(tx.Receiver())
				line += fmt.Sprintf("Send -%.2f to %s", tx.Amount(), toName)
			} else {
				// Received transaction
				fromName := m.getAccountDisplayName(tx.Sender())
				line += fmt.Sprintf("Receive +%.2f from %s", tx.Amount(), fromName)
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
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"Error: "+err.Error(),
			"",
			helpStyle.Render("esc: back"),
		)
	}

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
	var transactions []*scalegraph.Transaction
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
	if tx.Sender() == nil {
		// Mint transaction
		detailContent += "  Mint (Initial Balance)\n\n"
		detailContent += lipgloss.NewStyle().Bold(true).Render("To:") + "\n"
		detailContent += fmt.Sprintf("  %s\n", m.getAccountDisplayName(selectedAcc))
		detailContent += fmt.Sprintf("  %s\n\n", selectedAcc.ID().String())
	} else if tx.Sender().ID() == selectedAcc.ID() {
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

	// Amount
	detailContent += lipgloss.NewStyle().Bold(true).Render("Amount:") + "\n"
	amountStr := fmt.Sprintf("  %.2f", tx.Amount())
	if tx.Sender() == nil || tx.Sender().ID() != selectedAcc.ID() {
		amountStr = "  +" + amountStr
	} else {
		amountStr = "  -" + amountStr
	}
	detailContent += amountStr + "\n"

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
	accounts, err := m.app.GetAccounts(context.Background())
	if err != nil {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			"Error: "+err.Error(),
			"",
			helpStyle.Render("esc: back"),
		)
	}

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
				acc.ID().String()[:8]+"...",
				acc.Balance())
			if i == m.sendFromIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	case 1: // Select to account
		content = fmt.Sprintf("From: %s\n\n", accounts[m.sendFromIndex].ID().String()[:8]+"...")
		content += "Select TO account:\n\n"
		for i, acc := range accounts {
			if i == m.sendFromIndex {
				continue
			}
			cursor := "  "
			line := fmt.Sprintf("%s  Balance: %.2f",
				acc.ID().String()[:8]+"...",
				acc.Balance())
			if i == m.sendToIndex {
				cursor = "> "
				line = selectedStyle.Render(line)
			}
			content += cursor + line + "\n"
		}
	case 2: // Enter amount
		content = fmt.Sprintf("From: %s\n", accounts[m.sendFromIndex].ID().String()[:8]+"...")
		content += fmt.Sprintf("To: %s\n\n", accounts[m.sendToIndex].ID().String()[:8]+"...")
		content += "Enter amount: " + m.sendAmount + "█"
	}

	help := helpStyle.Render("↑/↓: navigate • enter: confirm • esc: back")

	status := ""
	if m.statusMsg != "" {
		status = statusStyle.Render(m.statusMsg)
	}

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

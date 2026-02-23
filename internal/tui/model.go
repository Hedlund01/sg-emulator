package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server"
)

// ViewState represents which screen is currently displayed
type ViewState int

const (
	ViewMenu ViewState = iota
	ViewCreateAccount
	ViewListAccounts
	ViewAccountDetail
	ViewAccountDetailSingle
	ViewTransactionDetail
	ViewSendMoney
	ViewVirtualNodes
	ViewTokenMenu
	ViewMintToken
	ViewAuthorizeTokenTransfer
	ViewTransferToken
)

// AccountCredentials caches loaded cryptographic credentials
type AccountCredentials struct {
	PrivateKeyPEM  string
	CertificatePEM string
}

// Model represents the TUI state
type Model struct {
	app    *server.Client
	server *server.Server

	width  int
	height int
	ready  bool
	view   ViewState

	// Account names (TUI-only, maps account ID to custom name)
	accountNames map[scalegraph.ScalegraphId]string

	// Credential cache (maps account ID to loaded credentials)
	credentialCache map[scalegraph.ScalegraphId]*AccountCredentials

	// Menu state
	menuCursor int

	// Create account state
	balanceInput       textinput.Model
	nameInput          textinput.Model
	pendingAccountID   scalegraph.ScalegraphId // ID of account being named
	createAccountFocus int                     // 0=balance, 1=name, 2=submit

	// Account selection state
	selectedAccountIndex     int
	transactionOffset        int // Scroll offset for transaction history
	selectedTransactionIndex int // Selected transaction in the list
	accountList              list.Model
	transactionList          list.Model

	// Send money state
	sendFromIndex int
	sendToIndex   int
	sendAmount    string
	sendStep      int // 0=from, 1=to, 2=amount

	// Token operations state
	tokenMenuCursor     int
	tokenStep           int
	tokenAccountIndex   int // from-account or single-account selector
	tokenToAccountIndex int // to-account selector (TransferToken step 2)
	tokenClawbackIndex  int // 0 = "No clawback", 1+ = account index (MintToken step 2)
	tokenTokenIndex     int // selected token index within an account's token list
	tokenValueInput     textinput.Model

	// Status message
	statusMsg string
}

// NewModel creates and returns a new Model instance
func NewModel(application *server.Client, srv *server.Server) Model {
	// Initialize balance input
	balanceInput := textinput.New()
	balanceInput.Placeholder = "0.00"
	balanceInput.CharLimit = 20
	balanceInput.Width = 30

	// Initialize name input
	nameInput := textinput.New()
	nameInput.Placeholder = "Adam"
	nameInput.CharLimit = 50
	nameInput.Width = 40

	// Initialize token value input
	tokenValueInput := textinput.New()
	tokenValueInput.Placeholder = "e.g. gold"
	tokenValueInput.CharLimit = 100
	tokenValueInput.Width = 40

	return Model{
		app:             application,
		server:          srv,
		view:            ViewMenu,
		accountNames:    make(map[scalegraph.ScalegraphId]string),
		credentialCache: make(map[scalegraph.ScalegraphId]*AccountCredentials),
		balanceInput:    balanceInput,
		nameInput:       nameInput,
		tokenValueInput: tokenValueInput,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// getAccountDisplayName returns the custom name or truncated ID for an account
func (m Model) getAccountDisplayName(acc *scalegraph.Account) string {
	if name, ok := m.accountNames[acc.ID()]; ok && name != "" {
		return name
	}
	return acc.ID().String()[:8] + "..."
}

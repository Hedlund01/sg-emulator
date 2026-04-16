package server

import (
	"time"

	eventv1 "sg-emulator/gen/event/v1"
	"sg-emulator/internal/scalegraph"
)

// eventInvolvedAccounts extracts all account IDs involved in an event.
func eventInvolvedAccounts(event *eventv1.Event) []string {
	switch d := event.GetData().(type) {
	case *eventv1.Event_Transfer:
		return []string{d.Transfer.GetFrom(), d.Transfer.GetTo()}
	case *eventv1.Event_Mint:
		return []string{d.Mint.GetTo()}
	case *eventv1.Event_MintToken:
		accs := []string{d.MintToken.GetAccountId()}
		if cb := d.MintToken.GetClawbackAddress(); cb != "" {
			accs = append(accs, cb)
		}
		return accs
	case *eventv1.Event_TransferToken:
		return []string{d.TransferToken.GetFrom(), d.TransferToken.GetTo()}
	case *eventv1.Event_AuthorizeTokenTransfer:
		return []string{d.AuthorizeTokenTransfer.GetAccountId(), d.AuthorizeTokenTransfer.GetTokenOwnerId()}
	case *eventv1.Event_UnauthorizeTokenTransfer:
		return []string{d.UnauthorizeTokenTransfer.GetAccountId(), d.UnauthorizeTokenTransfer.GetTokenOwnerId()}
	case *eventv1.Event_BurnToken:
		return []string{d.BurnToken.GetAccountId()}
	case *eventv1.Event_ClawbackToken:
		return []string{d.ClawbackToken.GetFrom(), d.ClawbackToken.GetTo()}
	case *eventv1.Event_FreezeToken:
		return []string{d.FreezeToken.GetFreezeAuthority(), d.FreezeToken.GetTokenHolder()}
	case *eventv1.Event_UnfreezeToken:
		return []string{d.UnfreezeToken.GetFreezeAuthority(), d.UnfreezeToken.GetTokenHolder()}
	default:
		return nil
	}
}

// BuildEvent constructs a proto Event from an event info struct.
// Returns nil if the info type is not recognized.
func BuildEvent(requestPayload any) *eventv1.Event {
	now := time.Now().Unix()

	switch req := requestPayload.(type) {
	case *transferEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_TRANSFER,
			Timestamp: now,
			Data: &eventv1.Event_Transfer{
				Transfer: &eventv1.TransferEventData{
					From:   req.From,
					To:     req.To,
					Amount: float64(req.Amount),
				},
			},
		}
	case *mintEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_MINT,
			Timestamp: now,
			Data: &eventv1.Event_Mint{
				Mint: &eventv1.MintEventData{
					To:     req.To,
					Amount: float64(req.Amount),
				},
			},
		}
	case *mintTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_MINT_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_MintToken{
				MintToken: &eventv1.MintTokenEventData{
					AccountId:       req.AccountID,
					TokenId:         req.TokenID,
					TokenValue:      req.TokenValue,
					ClawbackAddress: req.ClawbackAddress,
				},
			},
		}
	case *transferTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_TRANSFER_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_TransferToken{
				TransferToken: &eventv1.TransferTokenEventData{
					From:    req.From,
					To:      req.To,
					TokenId: req.TokenID,
				},
			},
		}
	case *authorizeTokenTransferEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_AUTHORIZE_TOKEN_TRANSFER,
			Timestamp: now,
			Data: &eventv1.Event_AuthorizeTokenTransfer{
				AuthorizeTokenTransfer: &eventv1.AuthorizeTokenTransferEventData{
					AccountId:    req.AccountID,
					TokenId:      req.TokenID,
					TokenOwnerId: req.TokenOwnerID,
				},
			},
		}
	case *unauthorizeTokenTransferEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_UNAUTHORIZE_TOKEN_TRANSFER,
			Timestamp: now,
			Data: &eventv1.Event_UnauthorizeTokenTransfer{
				UnauthorizeTokenTransfer: &eventv1.UnauthorizeTokenTransferEventData{
					AccountId:    req.AccountID,
					TokenId:      req.TokenID,
					TokenOwnerId: req.TokenOwnerID,
				},
			},
		}
	case *burnTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_BURN_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_BurnToken{
				BurnToken: &eventv1.BurnTokenEventData{
					AccountId: req.AccountID,
					TokenId:   req.TokenID,
				},
			},
		}
	case *clawbackTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_CLAWBACK_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_ClawbackToken{
				ClawbackToken: &eventv1.ClawbackTokenEventData{
					From:    req.From,
					To:      req.To,
					TokenId: req.TokenID,
				},
			},
		}
	case *freezeTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_FREEZE_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_FreezeToken{
				FreezeToken: &eventv1.FreezeTokenEventData{
					FreezeAuthority: req.FreezeAuthority,
					TokenHolder:     req.TokenHolder,
					TokenId:         req.TokenID,
				},
			},
		}
	case *unfreezeTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_UNFREEZE_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_UnfreezeToken{
				UnfreezeToken: &eventv1.UnfreezeTokenEventData{
					FreezeAuthority: req.FreezeAuthority,
					TokenHolder:     req.TokenHolder,
					TokenId:         req.TokenID,
				},
			},
		}
	default:
		return nil
	}
}

// extractEventInfo maps a domain request payload to an event info struct
// that BuildEvent can convert into a proto Event.
func extractEventInfo(requestPayload any, responsePayload any) any {
	switch req := requestPayload.(type) {
	case *scalegraph.TransferRequest:
		return &transferEventInfo{
			From:   req.From.String(),
			To:     req.To.String(),
			Amount: req.Amount,
		}
	case *scalegraph.MintRequest:
		return &mintEventInfo{
			To:     req.To.String(),
			Amount: req.Amount,
		}
	case *scalegraph.MintTokenRequest:
		info := &mintTokenEventInfo{
			AccountID:  req.SignedEnvelope.Signature.SignerID,
			TokenValue: req.TokenValue,
		}
		if req.ClawbackAddress != nil {
			info.ClawbackAddress = req.ClawbackAddress.String()
		}
		if resp, ok := responsePayload.(*scalegraph.MintTokenResponse); ok {
			info.TokenID = resp.TokenID
		}
		return info
	case *scalegraph.TransferTokenRequest:
		return &transferTokenEventInfo{
			From:    req.From.String(),
			To:      req.To.String(),
			TokenID: req.TokenId,
		}
	case *scalegraph.AuthorizeTokenTransferRequest:
		return &authorizeTokenTransferEventInfo{
			AccountID:    req.AccountID.String(),
			TokenID:      req.TokenId,
			TokenOwnerID: req.TokenOwnerID.String(),
		}
	case *scalegraph.UnauthorizeTokenTransferRequest:
		return &unauthorizeTokenTransferEventInfo{
			AccountID:    req.AccountID.String(),
			TokenID:      req.TokenId,
			TokenOwnerID: req.TokenOwnerID.String(),
		}
	case *scalegraph.BurnTokenRequest:
		return &burnTokenEventInfo{
			AccountID: req.AccountID.String(),
			TokenID:   req.TokenId,
		}
	case *scalegraph.ClawbackTokenRequest:
		return &clawbackTokenEventInfo{
			From:    req.From.String(),
			To:      req.To.String(),
			TokenID: req.TokenId,
		}
	case *scalegraph.FreezeTokenRequest:
		return &freezeTokenEventInfo{
			FreezeAuthority: req.FreezeAuthority.String(),
			TokenHolder:     req.TokenHolder.String(),
			TokenID:         req.TokenId,
		}
	case *scalegraph.UnfreezeTokenRequest:
		return &unfreezeTokenEventInfo{
			FreezeAuthority: req.FreezeAuthority.String(),
			TokenHolder:     req.TokenHolder.String(),
			TokenID:         req.TokenId,
		}
	default:
		return nil
	}
}

// Event info types used to construct events from domain request payloads.

type transferEventInfo struct {
	From   string
	To     string
	Amount float64
}

type mintEventInfo struct {
	To     string
	Amount float64
}

type mintTokenEventInfo struct {
	AccountID       string
	TokenID         string
	TokenValue      string
	ClawbackAddress string
}

type transferTokenEventInfo struct {
	From    string
	To      string
	TokenID string
}

type authorizeTokenTransferEventInfo struct {
	AccountID    string
	TokenID      string
	TokenOwnerID string
}

type unauthorizeTokenTransferEventInfo struct {
	AccountID    string
	TokenID      string
	TokenOwnerID string
}

type burnTokenEventInfo struct {
	AccountID string
	TokenID   string
}

type clawbackTokenEventInfo struct {
	From    string
	To      string
	TokenID string
}

type freezeTokenEventInfo struct {
	FreezeAuthority string
	TokenHolder     string
	TokenID         string
}

type unfreezeTokenEventInfo struct {
	FreezeAuthority string
	TokenHolder     string
	TokenID         string
}

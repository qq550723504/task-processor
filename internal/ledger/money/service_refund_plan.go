package money

const (
	ServiceReturnPre          = "RETURN_PRE"
	ServiceRefundPhase        = "REFUND"
	ServiceReturnPost         = "RETURN_POST"
	ServiceRefundReleasePhase = "REFUND_RELEASE"
)

// One immutable original refund plan, sealed under the payment binding lock.
// It carries no additional balance, entitlement, or channel transaction.
type ServiceRefundPlan struct {
	OrderID, PaymentID, CommandID, SourceProofID                                            string
	PreOperationID, RefundOperationID, PostOperationID, ReleaseOperationID, OriginalShareID string
	Allocation                                                                              ServiceAllocationPolicy
	NominalMinor, BaseRefundedMinor, BaseSettlementRefundedMinor, BaseReturnedMinor         int64
	SettlementMinor, NetShareMinor, RemainingNoCashMinor, PreReturnMinor                    int64
}

func NewServiceRefundPlan(f ServiceFundsView, command, source, pre, refund, post, release, share string, amount int64) (ServiceRefundPlan, error) {
	p := ServiceRefundPlan{OrderID: f.OrderID, PaymentID: f.PaymentID, CommandID: command, SourceProofID: source, PreOperationID: pre, RefundOperationID: refund, PostOperationID: post, ReleaseOperationID: release, OriginalShareID: share, Allocation: f.Allocation, NominalMinor: amount, BaseRefundedMinor: f.RefundedMinor, BaseSettlementRefundedMinor: f.SettlementRefundedMinor, BaseReturnedMinor: f.ReturnedMinor, SettlementMinor: f.SettlementMinor - f.SettlementRefundedMinor, NetShareMinor: f.SharedMinor - f.ReturnedMinor}
	if f.Allocation.Basis != ServiceAllocationChannelNetFloorV2 || f.ChannelAmounts == nil || f.ChannelAmounts.Validate(f.GrossMinor) != nil || f.ReconciliationReason != "" || f.ChargedBackMinor != 0 || amount <= 0 || amount > f.GrossMinor-f.RefundedMinor {
		return p, ErrConflict
	}
	for _, v := range f.ChannelAmounts.Vouchers {
		used := f.VoucherRefundedMinor[v.ID]
		if used < 0 || used > v.AmountMinor {
			return p, ErrConflict
		}
		if v.FundingType == "NOCASH" {
			p.RemainingNoCashMinor += v.AmountMinor - used
		}
	}
	if f.SharedMinor > 0 {
		if share == "" || p.NetShareMinor != f.PlatformMinor {
			return p, ErrConflict
		}
		lower := amount - p.RemainingNoCashMinor
		if lower < 0 {
			lower = 0
		}
		target, _, err := ServiceAllocation(p.SettlementMinor, lower, p.Allocation)
		if err != nil {
			return p, err
		}
		p.PreReturnMinor = p.NetShareMinor - target
	} else {
		p.OriginalShareID = ""
	}
	return p, p.Validate()
}
func (p ServiceRefundPlan) Validate() error {
	for _, id := range []string{p.OrderID, p.PaymentID, p.CommandID, p.SourceProofID, p.PreOperationID, p.RefundOperationID, p.PostOperationID, p.ReleaseOperationID} {
		if !isCanonicalWalletIdentifier(id) {
			return ErrInvalid
		}
	}
	ids := map[string]bool{}
	for _, id := range []string{p.PreOperationID, p.RefundOperationID, p.PostOperationID, p.ReleaseOperationID} {
		if ids[id] {
			return ErrInvalid
		}
		ids[id] = true
	}
	if p.Allocation.Validate() != nil || p.Allocation.Basis != ServiceAllocationChannelNetFloorV2 || p.NominalMinor <= 0 || p.BaseRefundedMinor < 0 || p.BaseSettlementRefundedMinor < 0 || p.BaseReturnedMinor < 0 || p.SettlementMinor < 0 || p.NetShareMinor < 0 || p.RemainingNoCashMinor < 0 || p.PreReturnMinor < 0 || p.PreReturnMinor > p.NetShareMinor {
		return ErrInvalid
	}
	if p.OriginalShareID == "" {
		if p.PreReturnMinor != 0 || p.NetShareMinor != 0 {
			return ErrInvalid
		}
	} else if !isCanonicalWalletIdentifier(p.OriginalShareID) {
		return ErrInvalid
	}
	return nil
}
func (p ServiceRefundPlan) PostReturn(actual ServiceRefundAmounts) (int64, error) {
	if p.Validate() != nil {
		return 0, ErrInvalid
	}
	if p.OriginalShareID == "" {
		return 0, nil
	}
	target, _, err := ServiceAllocation(p.SettlementMinor, actual.SettlementMinor(), p.Allocation)
	if err != nil {
		return 0, err
	}
	due := p.NetShareMinor - p.PreReturnMinor - target
	if due < 0 || due > p.NetShareMinor-p.PreReturnMinor {
		return 0, ErrConflict
	}
	return due, nil
}

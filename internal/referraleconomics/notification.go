package referraleconomics

import "context"

type NoticeReader interface {
	ListNoticeAdjustments(context.Context, string, string, int) ([]EarningsLedgerEntry, string, error)
	ListNoticeWithdrawals(context.Context, string, string, int) ([]Withdrawal, string, error)
}

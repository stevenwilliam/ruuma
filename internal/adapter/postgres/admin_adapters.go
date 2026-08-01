package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/app/adminsvc"
	"github.com/stevenwilliam/ruuma/internal/app/notifysvc"
	"github.com/stevenwilliam/ruuma/internal/app/ports"
)

// StoreWritePort implements adminsvc.StoreWrite over StoreRepo.
type StoreWritePort struct {
	repo *StoreRepo
	db   *gorm.DB
}

func NewStoreWritePort(repo *StoreRepo, db *gorm.DB) *StoreWritePort {
	return &StoreWritePort{repo: repo, db: db}
}

var _ adminsvc.StoreWrite = (*StoreWritePort)(nil)

func (p *StoreWritePort) Create(ctx context.Context, in ports.StoreView) (*ports.StoreView, error) {
	s := Store{
		ID: uuid.New(), Code: in.Code, Name: in.Name, Slug: in.Slug,
		AddressLine: in.AddressLine, City: in.City, Phone: in.Phone,
		Timezone: in.Timezone, IsActive: false, // activated deliberately after setup
	}
	if err := p.repo.Create(ctx, &s); err != nil {
		return nil, err
	}
	v := toStoreView(s, nil)
	return &v, nil
}

func (p *StoreWritePort) Update(ctx context.Context, in ports.StoreView) error {
	s, err := p.repo.Get(ctx, in.ID)
	if err != nil {
		return err
	}
	s.Code, s.Name, s.Slug = in.Code, in.Name, in.Slug
	s.AddressLine, s.City, s.Phone = in.AddressLine, in.City, in.Phone
	if in.Timezone != "" {
		s.Timezone = in.Timezone
	}
	return p.repo.Update(ctx, s)
}

func (p *StoreWritePort) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	return p.repo.SetActive(ctx, id, active)
}

func (p *StoreWritePort) ReplaceModes(ctx context.Context, storeID uuid.UUID, modes []string) error {
	return p.repo.ReplaceModes(ctx, storeID, modes)
}

func (p *StoreWritePort) Hours(ctx context.Context, storeID uuid.UUID) ([]adminsvc.HoursRow, error) {
	rows, err := p.repo.Hours(ctx, storeID)
	if err != nil {
		return nil, err
	}
	out := make([]adminsvc.HoursRow, 0, len(rows))
	for _, h := range rows {
		out = append(out, adminsvc.HoursRow{
			ID: h.ID, Weekday: h.Weekday, Mode: h.FulfilmentType, BlockIndex: h.BlockIndex,
			IsClosed: h.IsClosed, OpensAt: str(h.OpensAt), ClosesAt: str(h.ClosesAt),
		})
	}
	return out, nil
}

func (p *StoreWritePort) ReplaceHours(ctx context.Context, storeID uuid.UUID, rows []adminsvc.HoursRow) error {
	hours := make([]StoreHour, 0, len(rows))
	for _, r := range rows {
		h := StoreHour{
			Weekday: r.Weekday, FulfilmentType: r.Mode,
			BlockIndex: r.BlockIndex, IsClosed: r.IsClosed,
		}
		if !r.IsClosed {
			h.OpensAt, h.ClosesAt = strPtr(r.OpensAt), strPtr(r.ClosesAt)
		}
		hours = append(hours, h)
	}
	return p.repo.ReplaceHours(ctx, storeID, hours)
}

func (p *StoreWritePort) DateOverrides(ctx context.Context, storeID uuid.UUID, from, to time.Time) ([]adminsvc.OverrideRow, error) {
	rows, err := p.repo.DateOverrides(ctx, storeID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]adminsvc.OverrideRow, 0, len(rows))
	for _, o := range rows {
		out = append(out, adminsvc.OverrideRow{
			ID: o.ID, StoreID: o.StoreID, Date: o.BusinessDate, Mode: o.FulfilmentType,
			BlockIndex: o.BlockIndex, IsClosed: o.IsClosed,
			OpensAt: str(o.OpensAt), ClosesAt: str(o.ClosesAt), Reason: str(o.Reason),
		})
	}
	return out, nil
}

func (p *StoreWritePort) UpsertDateOverride(ctx context.Context, row adminsvc.OverrideRow, actor uuid.UUID) error {
	o := StoreDateOverride{
		ID: row.ID, StoreID: row.StoreID, BusinessDate: row.Date,
		FulfilmentType: row.Mode, BlockIndex: row.BlockIndex, IsClosed: row.IsClosed,
		Reason: strPtr(row.Reason), CreatedBy: &actor,
	}
	if !row.IsClosed {
		o.OpensAt, o.ClosesAt = strPtr(row.OpensAt), strPtr(row.ClosesAt)
	}
	return p.repo.UpsertDateOverride(ctx, &o)
}

func (p *StoreWritePort) DeleteDateOverride(ctx context.Context, storeID, id uuid.UUID) error {
	return p.repo.DeleteDateOverride(ctx, storeID, id)
}

func (p *StoreWritePort) Blackouts(ctx context.Context, storeID uuid.UUID, from, to time.Time) ([]adminsvc.BlackoutRow, error) {
	rows, err := p.repo.Blackouts(ctx, storeID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]adminsvc.BlackoutRow, 0, len(rows))
	for _, b := range rows {
		out = append(out, adminsvc.BlackoutRow{
			ID: b.ID, StoreID: b.StoreID, Date: b.BusinessDate, Reason: b.Reason,
		})
	}
	return out, nil
}

func (p *StoreWritePort) AddBlackout(ctx context.Context, row adminsvc.BlackoutRow, actor uuid.UUID) error {
	return p.repo.AddBlackout(ctx, &StoreBlackoutDate{
		StoreID: row.StoreID, BusinessDate: row.Date, Reason: row.Reason, CreatedBy: &actor,
	})
}

func (p *StoreWritePort) RemoveBlackout(ctx context.Context, storeID uuid.UUID, date time.Time) error {
	return p.repo.RemoveBlackout(ctx, storeID, date)
}

func (p *StoreWritePort) BankAccounts(ctx context.Context, storeID uuid.UUID) ([]ports.BankAccountView, error) {
	rows, err := p.repo.BankAccounts(ctx, storeID)
	if err != nil {
		return nil, err
	}
	out := make([]ports.BankAccountView, 0, len(rows))
	for _, a := range rows {
		out = append(out, ports.BankAccountView{
			ID: a.ID, BankName: a.BankName, AccountName: a.AccountName,
			AccountNumber: a.AccountNumber,
		})
	}
	return out, nil
}

func (p *StoreWritePort) SaveBankAccount(ctx context.Context, storeID uuid.UUID, in adminsvc.BankAccountInput) error {
	row := StoreBankAccount{
		ID: in.ID, StoreID: storeID, BankName: in.BankName, AccountName: in.AccountName,
		AccountNumber: in.AccountNumber, IsActive: in.IsActive,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if err := p.db.WithContext(ctx).Save(&row).Error; err != nil {
		return err
	}
	if in.IsPrimary {
		return p.repo.SetPrimaryBankAccount(ctx, storeID, row.ID)
	}
	return nil
}

func (p *StoreWritePort) SetPrimaryBankAccount(ctx context.Context, storeID, accountID uuid.UUID) error {
	return p.repo.SetPrimaryBankAccount(ctx, storeID, accountID)
}

// MenuWritePort implements adminsvc.MenuWrite over CatalogRepo.
type MenuWritePort struct {
	repo *CatalogRepo
	db   *gorm.DB
}

func NewMenuWritePort(repo *CatalogRepo, db *gorm.DB) *MenuWritePort {
	return &MenuWritePort{repo: repo, db: db}
}

var _ adminsvc.MenuWrite = (*MenuWritePort)(nil)

func (p *MenuWritePort) SaveCategory(ctx context.Context, in adminsvc.CategoryInput) (uuid.UUID, error) {
	c := Category{
		ID: in.ID, NameID: in.NameID, NameEN: in.NameEN, Slug: in.Slug,
		Cuisine: in.Cuisine, SortOrder: in.SortOrder, IsActive: in.IsActive,
	}
	if c.ID == uuid.Nil {
		if err := p.repo.CreateCategory(ctx, &c); err != nil {
			return uuid.Nil, err
		}
		return c.ID, nil
	}
	return c.ID, p.repo.UpdateCategory(ctx, &c)
}

func (p *MenuWritePort) SaveItem(ctx context.Context, in adminsvc.ItemInput) (uuid.UUID, error) {
	item := MenuItem{
		ID: in.ID, CategoryID: in.CategoryID, SKU: in.SKU,
		NameID: in.NameID, NameEN: in.NameEN,
		DescriptionID: strPtr(in.DescriptionID), DescriptionEN: strPtr(in.DescriptionEN),
		BasePrice: in.BasePrice, KitchenUnits: in.KitchenUnits, PrepMinutes: in.PrepMinutes,
		MinLeadMinutes: in.MinLeadMinutes, SpiceLevel: in.SpiceLevel,
		IsHalal: in.IsHalal, IsVegetarian: in.IsVegetarian, ContainsPork: in.ContainsPork,
		ContainsAlcohol: in.ContainsAlcohol, ContainsNuts: in.ContainsNuts,
		IsActive: in.IsActive, SortOrder: in.SortOrder,
	}
	if item.KitchenUnits <= 0 {
		item.KitchenUnits = 1
	}
	if item.ID == uuid.Nil {
		if err := p.repo.CreateItem(ctx, &item); err != nil {
			return uuid.Nil, err
		}
		return item.ID, nil
	}
	// Preserve the photo when an edit does not carry one.
	existing, err := p.repo.GetItem(ctx, item.ID)
	if err != nil {
		return uuid.Nil, err
	}
	item.PhotoKey = existing.PhotoKey
	item.CreatedAt = existing.CreatedAt
	return item.ID, p.repo.UpdateItem(ctx, &item)
}

func (p *MenuWritePort) SetStoreOverride(ctx context.Context, storeID, itemID uuid.UUID,
	available *bool, price *int64, actor uuid.UUID) error {
	return p.repo.SetStoreOverride(ctx, storeID, itemID, available, price, actor)
}

func (p *MenuWritePort) Add86(ctx context.Context, storeID, itemID uuid.UUID, until *time.Time, reason string, actor uuid.UUID) error {
	return p.repo.Add86(ctx, storeID, itemID, until, reason, actor)
}

func (p *MenuWritePort) Lift86(ctx context.Context, storeID, itemID uuid.UUID) error {
	return p.repo.Lift86(ctx, storeID, itemID)
}

func (p *MenuWritePort) SetDailyStock(ctx context.Context, storeID, itemID uuid.UUID, date time.Time, total int) error {
	return p.repo.SetDailyStock(ctx, storeID, itemID, date, total)
}

func (p *MenuWritePort) SetItemPhoto(ctx context.Context, itemID uuid.UUID, objectKey string) error {
	return p.db.WithContext(ctx).Model(&MenuItem{}).Where("id = ?", itemID).
		Updates(map[string]any{"photo_key": objectKey, "updated_at": time.Now()}).Error
}

// ParamWritePort implements adminsvc.ParamWrite over ParamRepo.
type ParamWritePort struct{ repo *ParamRepo }

func NewParamWritePort(repo *ParamRepo) *ParamWritePort { return &ParamWritePort{repo: repo} }

var _ adminsvc.ParamWrite = (*ParamWritePort)(nil)

func (p *ParamWritePort) ListGroup(ctx context.Context, q string) ([]adminsvc.ParamRow, error) {
	rows, err := p.repo.ListGroup(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]adminsvc.ParamRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, adminsvc.ParamRow{
			Key: r.Key, Value: r.Value, DataType: r.DataType, Description: str(r.Description),
			IsSecret: r.IsSecret, IsStoreOverridable: r.IsStoreOverridable, Source: "group",
		})
	}
	return out, nil
}

func (p *ParamWritePort) ListStore(ctx context.Context, storeID uuid.UUID) ([]adminsvc.ParamRow, error) {
	rows, err := p.repo.ListStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	out := make([]adminsvc.ParamRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, adminsvc.ParamRow{Key: r.Key, Value: r.Value, Source: "store"})
	}
	return out, nil
}

func (p *ParamWritePort) UpsertGroup(ctx context.Context, key, value string, actor uuid.UUID) error {
	return p.repo.UpsertGroup(ctx, key, value, actor)
}

func (p *ParamWritePort) UpsertStore(ctx context.Context, storeID uuid.UUID, key, value string, actor uuid.UUID) error {
	return p.repo.UpsertStore(ctx, storeID, key, value, actor)
}

func (p *ParamWritePort) DeleteGroup(ctx context.Context, key string) error {
	return p.repo.DeleteGroup(ctx, key)
}

func (p *ParamWritePort) DeleteStore(ctx context.Context, storeID uuid.UUID, key string) error {
	return p.repo.DeleteStore(ctx, storeID, key)
}

// AuditReadPort implements adminsvc.AuditRead over AuditRepo.
type AuditReadPort struct{ repo *AuditRepo }

func NewAuditReadPort(repo *AuditRepo) *AuditReadPort { return &AuditReadPort{repo: repo} }

var _ adminsvc.AuditRead = (*AuditReadPort)(nil)

func (p *AuditReadPort) List(ctx context.Context, q, entityType string, storeID *uuid.UUID,
	from, to *time.Time, limit int, scope []uuid.UUID) ([]adminsvc.AuditRow, error) {

	rows, err := p.repo.List(ctx, AuditFilter{
		Q: q, EntityType: entityType, StoreID: storeID, From: from, To: to, Limit: limit,
	}, scope)
	if err != nil {
		return nil, err
	}
	out := make([]adminsvc.AuditRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, adminsvc.AuditRow{
			ID: r.ID, ActorEmail: str(r.ActorEmail), ActorType: r.ActorType,
			Action: r.Action, EntityType: r.EntityType, EntityID: r.EntityID,
			StoreID: r.StoreID, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// NotifyQueuePort adapts the notification store to notifysvc.
type NotifyQueuePort struct{ repo *NotifyRepo }

func NewNotifyQueuePort(repo *NotifyRepo) *NotifyQueuePort { return &NotifyQueuePort{repo: repo} }

// Due returns queued notifications for the dispatcher.
func (p *NotifyQueuePort) Due(ctx context.Context, limit int) ([]notifysvc.Queued, error) {
	rows, err := p.repo.Due(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]notifysvc.Queued, 0, len(rows))
	for _, r := range rows {
		out = append(out, notifysvc.Queued{
			ID: r.ID, Target: r.Target, Body: r.Body,
			TemplateKey: r.TemplateKey, Language: r.Language, Attempts: r.Attempts,
		})
	}
	return out, nil
}

func (p *NotifyQueuePort) MarkSent(ctx context.Context, id uuid.UUID) error {
	return p.repo.MarkSent(ctx, id)
}

func (p *NotifyQueuePort) MarkFailed(ctx context.Context, id uuid.UUID, cause string, attempt int) error {
	return p.repo.MarkFailed(ctx, id, cause, attempt)
}

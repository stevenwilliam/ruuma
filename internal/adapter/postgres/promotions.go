package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/stevenwilliam/ruuma/internal/domain/money"
	"github.com/stevenwilliam/ruuma/internal/domain/pricing"
	"github.com/stevenwilliam/ruuma/internal/platform/apierror"
)

// PromoRepo loads promotions and their explicit store scope (D15, BR-2.5.8).
type PromoRepo struct{ db *gorm.DB }

func NewPromoRepo(db *gorm.DB) *PromoRepo { return &PromoRepo{db: db} }

// ByCode loads a promotion with its store list, category restriction and this
// customer's usage, ready for the pure evaluator.
func (r *PromoRepo) ByCode(ctx context.Context, code string, customerID uuid.UUID) (*Promotion, pricing.Promotion, error) {
	var p Promotion
	err := r.db.WithContext(ctx).Where("code = ?", strings.TrimSpace(code)).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pricing.Promotion{}, apierror.Unprocessable(apierror.CodePromoInvalid,
			"That promo code is not valid.")
	}
	if err != nil {
		return nil, pricing.Promotion{}, err
	}

	var storeIDs []uuid.UUID
	if err := r.db.WithContext(ctx).Model(&PromotionStore{}).
		Where("promotion_id = ?", p.ID).Pluck("store_id", &storeIDs).Error; err != nil {
		return nil, pricing.Promotion{}, err
	}
	var categoryIDs []uuid.UUID
	if err := r.db.WithContext(ctx).Model(&PromotionCategory{}).
		Where("promotion_id = ?", p.ID).Pluck("category_id", &categoryIDs).Error; err != nil {
		return nil, pricing.Promotion{}, err
	}

	var customerUsed int64
	if err := r.db.WithContext(ctx).Model(&PromotionRedemption{}).
		Where("promotion_id = ? AND customer_id = ? AND released_at IS NULL", p.ID, customerID).
		Count(&customerUsed).Error; err != nil {
		return nil, pricing.Promotion{}, err
	}

	dp := pricing.Promotion{
		Code:              p.Code,
		Type:              pricing.DiscountType(p.DiscountType),
		MinSpend:          money.Rupiah(p.MinSpend),
		StartsAt:          p.StartsAt,
		EndsAt:            p.EndsAt,
		IsActive:          p.IsActive,
		UsedCount:         p.UsedCount,
		CustomerUsedCount: int(customerUsed),
		StoreIDs:          toStrings(storeIDs),
		CategoryIDs:       toStrings(categoryIDs),
	}
	if p.ValueBps != nil {
		dp.ValueBps = money.Bps(*p.ValueBps)
	}
	if p.ValueAmount != nil {
		dp.ValueAmount = money.Rupiah(*p.ValueAmount)
	}
	if p.MaxDiscount != nil {
		dp.MaxDiscount = money.Rupiah(*p.MaxDiscount)
	}
	if p.UsageCapTotal != nil {
		dp.UsageCapTotal = *p.UsageCapTotal
	}
	if p.UsageCapPerCustomer != nil {
		dp.UsageCapPerCustomer = *p.UsageCapPerCustomer
	}
	return &p, dp, nil
}

// List returns promotions, searchable (BR-1.5.1).
func (r *PromoRepo) List(ctx context.Context, q string) ([]Promotion, error) {
	query := r.db.WithContext(ctx).Model(&Promotion{}).Order("starts_at DESC")
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("code ILIKE ? OR name ILIKE ?", like, like)
	}
	var out []Promotion
	return out, query.Find(&out).Error
}

func (r *PromoRepo) Get(ctx context.Context, id uuid.UUID) (*Promotion, error) {
	var p Promotion
	err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apierror.NotFound("Promotion not found.")
	}
	return &p, err
}

// Save writes a promotion with its store and category scope in one transaction.
// A promotion with no stores is refused: there is no implicit "all stores" (D15).
func (r *PromoRepo) Save(ctx context.Context, p *Promotion, storeIDs, categoryIDs []uuid.UUID) error {
	if len(storeIDs) == 0 {
		return apierror.Validation("A promotion must apply to at least one store.",
			map[string]any{"store_ids": "required"})
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(p).Error; err != nil {
			return err
		}
		if err := tx.Where("promotion_id = ?", p.ID).Delete(&PromotionStore{}).Error; err != nil {
			return err
		}
		for _, sid := range storeIDs {
			if err := tx.Create(&PromotionStore{ID: uuid.New(), PromotionID: p.ID, StoreID: sid}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("promotion_id = ?", p.ID).Delete(&PromotionCategory{}).Error; err != nil {
			return err
		}
		for _, cid := range categoryIDs {
			if err := tx.Create(&PromotionCategory{ID: uuid.New(), PromotionID: p.ID, CategoryID: cid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Stores returns a promotion's explicit store scope.
func (r *PromoRepo) Stores(ctx context.Context, promotionID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	return ids, r.db.WithContext(ctx).Model(&PromotionStore{}).
		Where("promotion_id = ?", promotionID).Pluck("store_id", &ids).Error
}

// Categories returns a promotion's category restriction.
func (r *PromoRepo) Categories(ctx context.Context, promotionID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	return ids, r.db.WithContext(ctx).Model(&PromotionCategory{}).
		Where("promotion_id = ?", promotionID).Pluck("category_id", &ids).Error
}

func (r *PromoRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&Promotion{}).Error
}

func toStrings(in []uuid.UUID) []string {
	out := make([]string, len(in))
	for i, u := range in {
		out[i] = u.String()
	}
	return out
}

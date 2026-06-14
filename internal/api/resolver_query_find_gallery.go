package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

func (r *queryResolver) FindGallery(ctx context.Context, id string) (ret *models.Gallery, err error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Gallery.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}

	if ret != nil && isPathBlockedInRestrictedMode(ctx, ret.Path) {
		return nil, nil
	}

	return ret, nil
}

func (r *queryResolver) FindGalleries(ctx context.Context, galleryFilter *models.GalleryFilterType, filter *models.FindFilterType, ids []string) (ret *FindGalleriesResultType, err error) {
	idInts, err := handleIDList(ids, "ids")
	if err != nil {
		return nil, err
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var galleries []*models.Gallery
		var err error
		var total int

		if len(idInts) > 0 {
			galleries, err = r.repository.Gallery.FindMany(ctx, idInts)
			total = len(galleries)
		} else {
			galleries, total, err = r.repository.Gallery.Query(ctx, galleryFilter, filter)
		}

		if err != nil {
			return err
		}

		galleries = filterRestrictedGalleries(ctx, galleries)
		total = len(galleries)

		ret = &FindGalleriesResultType{
			Count:     total,
			Galleries: galleries,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *queryResolver) AllGalleries(ctx context.Context) (ret []*models.Gallery, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Gallery.All(ctx)
		ret = filterRestrictedGalleries(ctx, ret)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func filterRestrictedGalleries(ctx context.Context, galleries []*models.Gallery) []*models.Gallery {
	if !isSessionRestricted(ctx) {
		return galleries
	}

	filtered := make([]*models.Gallery, 0, len(galleries))
	for _, gallery := range galleries {
		if gallery == nil || isPathBlockedInRestrictedMode(ctx, gallery.Path) {
			continue
		}

		filtered = append(filtered, gallery)
	}

	return filtered
}

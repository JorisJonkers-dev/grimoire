package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// assetURL is the same-origin path of a picture, versioned by its content hash so caches stay correct.
func assetURL(id domain.CampaignID, ch domain.CharacterID, kind domain.ImageKind, key string) oas.AssetUrl {
	version := strings.TrimPrefix(key, "sha256/")
	version, _, _ = strings.Cut(version, ".")
	return oas.AssetUrl("/api/v1/campaigns/" + uuid.UUID(id).String() + "/characters/" + uuid.UUID(ch).String() + "/" +
		string(kind) + "?v=" + version[:min(12, len(version))])
}

const imageCache = "private, max-age=31536000, immutable"

func readImage(r io.Reader) ([]byte, bool) {
	data, err := io.ReadAll(io.LimitReader(r, app.MaxImageBytes+1))
	return data, err == nil && len(data) <= app.MaxImageBytes
}

func tooLarge() *oas.ProblemStatusCodeWithHeaders {
	return problem(http.StatusRequestEntityTooLarge, "Too large", "Pictures must be at most 10 MB.")
}

func (h *Handler) setImage(ctx context.Context, cid, chid oas.ID, kind domain.ImageKind, body io.Reader) *oas.ProblemStatusCodeWithHeaders {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized()
	}
	data, ok := readImage(body)
	if !ok {
		return tooLarge()
	}
	if err := h.Characters.SetImage(ctx, c, domain.CampaignID(cid), domain.CharacterID(chid), kind, data); err != nil {
		return h.campaignProblem(ctx, "set image", err)
	}
	return nil
}

func (h *Handler) image(ctx context.Context, cid, chid oas.ID, kind domain.ImageKind) (domain.Image, []byte, *oas.ProblemStatusCodeWithHeaders) {
	c, ok := uiCaller(ctx)
	if !ok {
		return domain.Image{}, nil, unauthorized()
	}
	img, data, err := h.Characters.Image(ctx, c, domain.CampaignID(cid), domain.CharacterID(chid), kind)
	if err != nil {
		return domain.Image{}, nil, h.campaignProblem(ctx, "get image", err)
	}
	return img, data, nil
}

// SetPortrait stores a Character's portrait.
func (h *Handler) SetPortrait(ctx context.Context, req oas.SetPortraitReq, p oas.SetPortraitParams) (oas.SetPortraitRes, error) {
	if prob := h.setImage(ctx, p.CampaignId, p.CharacterId, domain.Portrait, req.Data); prob != nil {
		return prob, nil
	}
	return &oas.SetPortraitNoContent{}, nil
}

// SetTokenIcon stores a Character's token icon.
func (h *Handler) SetTokenIcon(ctx context.Context, req oas.SetTokenIconReq, p oas.SetTokenIconParams) (oas.SetTokenIconRes, error) {
	if prob := h.setImage(ctx, p.CampaignId, p.CharacterId, domain.TokenIcon, req.Data); prob != nil {
		return prob, nil
	}
	return &oas.SetTokenIconNoContent{}, nil
}

// ClearTokenIcon switches a token back to initials.
func (h *Handler) ClearTokenIcon(ctx context.Context, p oas.ClearTokenIconParams) (oas.ClearTokenIconRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Characters.ClearToken(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId)); err != nil {
		return h.campaignProblem(ctx, "clear token", err), nil
	}
	return &oas.ClearTokenIconNoContent{}, nil
}

// GetPortrait serves a Character's portrait to Campaign members.
func (h *Handler) GetPortrait(ctx context.Context, p oas.GetPortraitParams) (oas.GetPortraitRes, error) {
	img, data, prob := h.image(ctx, p.CampaignId, p.CharacterId, domain.Portrait)
	if prob != nil {
		return prob, nil
	}
	cache, body := oas.NewOptString(imageCache), bytes.NewReader(data)
	switch img.Type {
	case "image/png":
		return &oas.GetPortraitOKImagePNGHeaders{CacheControl: cache, Response: oas.GetPortraitOKImagePNG{Data: body}}, nil
	case "image/webp":
		return &oas.GetPortraitOKImageWEBPHeaders{CacheControl: cache, Response: oas.GetPortraitOKImageWEBP{Data: body}}, nil
	default:
		return &oas.GetPortraitOKImageJpegHeaders{CacheControl: cache, Response: oas.GetPortraitOKImageJpeg{Data: body}}, nil
	}
}

// GetTokenIcon serves a Character's token icon to Campaign members.
func (h *Handler) GetTokenIcon(ctx context.Context, p oas.GetTokenIconParams) (oas.GetTokenIconRes, error) {
	img, data, prob := h.image(ctx, p.CampaignId, p.CharacterId, domain.TokenIcon)
	if prob != nil {
		return prob, nil
	}
	cache, body := oas.NewOptString(imageCache), bytes.NewReader(data)
	switch img.Type {
	case "image/png":
		return &oas.GetTokenIconOKImagePNGHeaders{CacheControl: cache, Response: oas.GetTokenIconOKImagePNG{Data: body}}, nil
	case "image/webp":
		return &oas.GetTokenIconOKImageWEBPHeaders{CacheControl: cache, Response: oas.GetTokenIconOKImageWEBP{Data: body}}, nil
	default:
		return &oas.GetTokenIconOKImageJpegHeaders{CacheControl: cache, Response: oas.GetTokenIconOKImageJpeg{Data: body}}, nil
	}
}

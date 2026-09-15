package scraper

import (
	"regexp"
	"strings"
)

var (
	promoSkuPrefixRegex   = regexp.MustCompile(`(?i)^(?:TK[A-Z]{2,6}[-_]?\d+|[A-Z]?[4679][A-Z]{2,6}[-_]?\d+|(?:77|88)[A-Z]{3,6}[-_]?\d+|S209|C209|E209)`)
	promoSkuSuffixRegex   = regexp.MustCompile(`(?i)(?:[-_]?TK\d*|[-_]?EC|-T-EC)$`)
	compilationTitleRegex = regexp.MustCompile(`(?i)\d+連発|\d+連射|\d+時間(?:BOX|ベスト)?|ベストセレクション|BESTセレクション|総集編|オムニバス|傑作選|名場面|全集|メモリアル|プレミアムベスト|\d+本番ベスト|\d+コーナー|\d+射精|大乱交(?:絶頂)?\d+本番|メモリアルベスト|コンプリートベスト|神BEST|\b(?:1st|2nd|3rd|4th|5th)?BEST\b|ベスト|[1-9]\d*人|[1-9]\d*体|(?:[3-9]\d{2,}|\d{4,})分`)
	goodsBundleRegex      = regexp.MustCompile(`(?i)パンティ|キーホルダー|購入特典|限定特典|オンラインサイン会|参加URL|参加権|チェキセット|チェキ付き?|ポラロイドセット|ポラロイド付き?|【(?:数量限定|FANZA限定|期間限定)】.*?(?:生写真|チェキ|写真|セット|付き?)`)
)

// IsPromotionalOrOmnibusVariant checks if a movie represents a promotional variant, goods bundle,
// non-video item, or multi-actress omnibus compilation.
// Returns (shouldSkip bool, reason string).
func IsPromotionalOrOmnibusVariant(movieID, title, originalTitle, coverURL string, genres []string, actressCount ...int) (bool, string) {
	upperID := strings.ToUpper(strings.TrimSpace(movieID))
	allText := strings.TrimSpace(title + " " + originalTitle)
	allLower := strings.ToLower(allText)

	// 1. Photobooks / Digital books / E-books
	if strings.Contains(coverURL, "ebook-assets") || strings.Contains(coverURL, "/e-book/") {
		return true, "Photobook / Digital Book"
	}
	if strings.HasPrefix(upperID, "B600") || strings.HasPrefix(upperID, "D600") || strings.HasPrefix(upperID, "DG") {
		return true, "Photobook / Digital Book"
	}
	for _, kw := range []string{"写真集", "デジタル写真集", "ポーズブック", "フォトブック", "電子書籍", "photobook", "photo book"} {
		if strings.Contains(allLower, kw) {
			return true, "Photobook / Digital Book"
		}
	}

	// 2. Promotional SKU prefixes & suffixes (e.g. 1DLDSS559TK, 1DLDSS545TK, TKCJOD-510, 9OFJE-548, MMR-AZ545TK)
	if promoSkuPrefixRegex.MatchString(upperID) || promoSkuSuffixRegex.MatchString(upperID) ||
		strings.HasSuffix(upperID, "-EC") || strings.HasSuffix(upperID, "-T-EC") || strings.HasSuffix(upperID, "EC") {
		return true, "Promotional SKU Variant"
	}

	// 3. Goods bundle markers (e.g. panties, cheki set, purchase bonus, limited edition bundle)
	if goodsBundleRegex.MatchString(allText) || strings.Contains(allLower, "cheki set") || strings.Contains(allLower, "polaroid set") {
		return true, "Promotional Bundle Variant"
	}

	// 4. Variety talk shows & known series (KCKC-, MLTN-, BMW-)
	if strings.HasPrefix(upperID, "KCKC") || strings.HasPrefix(upperID, "MLTN") || strings.HasPrefix(upperID, "BMW") ||
		strings.Contains(allText, "カチコチTV") || strings.Contains(allText, "カチコチ") {
		return true, "Variety / Talk Show"
	}

	// 5. Re-issue Director's Cut / Remaster duplicates (JQRE-, 未公開映像収録, etc.)
	if strings.Contains(allText, "未公開映像収録") ||
		strings.Contains(allText, "ディレクターズカット") ||
		strings.Contains(allText, "AIリマスター") ||
		strings.Contains(allLower, "ai remaster") ||
		strings.Contains(allText, "デジタルリマスター") ||
		strings.Contains(allLower, "digital remaster") ||
		strings.Contains(allText, "リマスター") ||
		strings.Contains(allLower, "remaster") ||
		strings.Contains(allText, "復刻") ||
		strings.HasPrefix(upperID, "JQRE") {
		return true, "Remaster / Director's Cut"
	}

	// 6. Genres (compilation, omnibus, set products, animation)
	for _, g := range genres {
		gNorm := strings.ToLower(strings.TrimSpace(g))
		if gNorm == "anime" || gNorm == "animation" || gNorm == "game" || gNorm == "comic" || gNorm == "manga" {
			return true, "Non-Video Media"
		}
		if gNorm == "photo book" || gNorm == "digital photo" || gNorm == "collection of photographs" {
			return true, "Photobook / Digital Book"
		}
		if gNorm == "special offers and set products" || strings.Contains(gNorm, "set products") {
			return true, "Promotional Bundle Variant"
		}
		if gNorm == "includes event participation rights" || strings.Contains(gNorm, "event participation") {
			return true, "Event / Autograph Session"
		}
		if gNorm == "compilation" || gNorm == "omnibus" {
			return true, "Omnibus Compilation"
		}
	}

	// 7. Known Omnibus Series (RBB-, MKCK-, MKMP-, OFJE-, SETH-, OFRF-, OFMA-)
	if strings.HasPrefix(upperID, "RBB") || strings.HasPrefix(upperID, "MKCK") || strings.HasPrefix(upperID, "MKMP") ||
		strings.HasPrefix(upperID, "OFJE") || strings.Contains(upperID, "OFJE") ||
		strings.HasPrefix(upperID, "SETH") || strings.Contains(upperID, "SETH") ||
		strings.HasPrefix(upperID, "OFRF") || strings.HasPrefix(upperID, "OFMA") {
		return true, "Omnibus Compilation"
	}

	// 8. Compilation / Best-Of / Multi-Person in title
	// Exemption: Official anniversary crossover works
	isAnniversary := strings.Contains(allText, "周年") ||
		strings.Contains(allLower, "anniversary") ||
		strings.Contains(allText, "記念作品") ||
		strings.Contains(allText, "創立")

	if !isAnniversary {
		if compilationTitleRegex.MatchString(allText) {
			return true, "Omnibus Compilation"
		}

		actCount := 0
		if len(actressCount) > 0 {
			actCount = actressCount[0]
		}
		if actCount >= 10 || strings.Contains(allText, "REbecca STARS") {
			return true, "Omnibus Compilation"
		}

		if actCount >= 5 {
			for _, g := range genres {
				if strings.EqualFold(strings.TrimSpace(g), "over 4 hours") {
					return true, "Omnibus Compilation"
				}
			}
			if strings.Contains(allLower, "600min") || strings.Contains(allLower, "480分") || strings.Contains(allLower, "300分") {
				return true, "Omnibus Compilation"
			}
		}
	}

	return false, ""
}

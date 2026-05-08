package service

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	openAIImageSizeMultiple = 16
	openAIImageMaxEdge      = 3840
	openAIImageMaxAspect    = 3.0
	openAIImageMinPixels    = 655_360
	openAIImageMaxPixels    = 8_294_400
)

var (
	openAIImageSizePattern = regexp.MustCompile(`^\s*(\d+)\s*[xX×]\s*(\d+)\s*$`)
	openAIImageRatioPattern = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*[:xX×]\s*(\d+(?:\.\d+)?)\s*$`)
)

type openAIImageTier string

const (
	openAIImageTier1K openAIImageTier = "1K"
	openAIImageTier2K openAIImageTier = "2K"
	openAIImageTier4K openAIImageTier = "4K"
)

func normalizeOpenAIImageSizeTier(value string) openAIImageTier {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case string(openAIImageTier1K), string(openAIImageTier2K), string(openAIImageTier4K):
		return openAIImageTier(strings.ToUpper(strings.TrimSpace(value)))
	default:
		return ""
	}
}

func normalizeOpenAIImageAspectRatio(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "auto") {
		return "auto"
	}

	trimmed = strings.ToLower(trimmed)
	trimmed = strings.ReplaceAll(trimmed, "×", "x")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	trimmed = strings.ReplaceAll(trimmed, "x", ":")
	return trimmed
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func roundToMultiple(value float64, multiple int) int {
	return maxInt(multiple, int(math.Round(value/float64(multiple)))*multiple)
}

func floorToMultiple(value float64, multiple int) int {
	return maxInt(multiple, int(math.Floor(value/float64(multiple)))*multiple)
}

func ceilToMultiple(value float64, multiple int) int {
	return maxInt(multiple, int(math.Ceil(value/float64(multiple)))*multiple)
}

func normalizeDimensions(width, height int) (int, int) {
	normalizedWidth := roundToMultiple(float64(width), openAIImageSizeMultiple)
	normalizedHeight := roundToMultiple(float64(height), openAIImageSizeMultiple)

	scaleToFit := func(scale float64) {
		normalizedWidth = floorToMultiple(float64(normalizedWidth)*scale, openAIImageSizeMultiple)
		normalizedHeight = floorToMultiple(float64(normalizedHeight)*scale, openAIImageSizeMultiple)
	}

	scaleToFill := func(scale float64) {
		normalizedWidth = ceilToMultiple(float64(normalizedWidth)*scale, openAIImageSizeMultiple)
		normalizedHeight = ceilToMultiple(float64(normalizedHeight)*scale, openAIImageSizeMultiple)
	}

	for i := 0; i < 4; i++ {
		maxEdge := maxInt(normalizedWidth, normalizedHeight)
		if maxEdge > openAIImageMaxEdge {
			scaleToFit(float64(openAIImageMaxEdge) / float64(maxEdge))
		}

		if float64(normalizedWidth)/float64(normalizedHeight) > openAIImageMaxAspect {
			normalizedWidth = floorToMultiple(float64(normalizedHeight)*openAIImageMaxAspect, openAIImageSizeMultiple)
		} else if float64(normalizedHeight)/float64(normalizedWidth) > openAIImageMaxAspect {
			normalizedHeight = floorToMultiple(float64(normalizedWidth)*openAIImageMaxAspect, openAIImageSizeMultiple)
		}

		pixels := normalizedWidth * normalizedHeight
		if pixels > openAIImageMaxPixels {
			scaleToFit(math.Sqrt(float64(openAIImageMaxPixels) / float64(pixels)))
		} else if pixels < openAIImageMinPixels {
			scaleToFill(math.Sqrt(float64(openAIImageMinPixels) / float64(pixels)))
		}
	}

	return normalizedWidth, normalizedHeight
}

func normalizeImageSize(size string) string {
	trimmed := strings.TrimSpace(size)
	match := openAIImageSizePattern.FindStringSubmatch(trimmed)
	if len(match) != 3 {
		return trimmed
	}

	width, err := strconv.Atoi(match[1])
	if err != nil {
		return trimmed
	}
	height, err := strconv.Atoi(match[2])
	if err != nil {
		return trimmed
	}

	normalizedWidth, normalizedHeight := normalizeDimensions(width, height)
	return fmt.Sprintf("%dx%d", normalizedWidth, normalizedHeight)
}

func parseRatio(ratio string) (float64, float64, bool) {
	match := openAIImageRatioPattern.FindStringSubmatch(strings.TrimSpace(ratio))
	if len(match) != 3 {
		return 0, 0, false
	}

	widthRatio, err := strconv.ParseFloat(match[1], 64)
	if err != nil || widthRatio <= 0 {
		return 0, 0, false
	}
	heightRatio, err := strconv.ParseFloat(match[2], 64)
	if err != nil || heightRatio <= 0 {
		return 0, 0, false
	}

	return widthRatio, heightRatio, true
}

func calculateOpenAIImageSize(tier openAIImageTier, ratio string) string {
	widthRatio, heightRatio, ok := parseRatio(ratio)
	if !ok {
		return ""
	}

	if widthRatio == heightRatio {
		side := 1024
		switch tier {
		case openAIImageTier1K:
			side = 1024
		case openAIImageTier2K:
			side = 2048
		case openAIImageTier4K:
			side = 3840
		default:
			return ""
		}
		return normalizeImageSize(fmt.Sprintf("%dx%d", side, side))
	}

	switch tier {
	case openAIImageTier1K:
		shortSide := 1024
		if widthRatio > heightRatio {
			width := roundToMultiple(float64(shortSide)*widthRatio/heightRatio, openAIImageSizeMultiple)
			return fmt.Sprintf("%dx%d", width, shortSide)
		}
		height := roundToMultiple(float64(shortSide)*heightRatio/widthRatio, openAIImageSizeMultiple)
		return fmt.Sprintf("%dx%d", shortSide, height)
	case openAIImageTier2K, openAIImageTier4K:
		longSide := 2048
		if tier == openAIImageTier4K {
			longSide = 3840
		}
		if widthRatio > heightRatio {
			height := roundToMultiple(float64(longSide)*heightRatio/widthRatio, openAIImageSizeMultiple)
			return normalizeImageSize(fmt.Sprintf("%dx%d", longSide, height))
		}
		width := roundToMultiple(float64(longSide)*widthRatio/heightRatio, openAIImageSizeMultiple)
		return normalizeImageSize(fmt.Sprintf("%dx%d", width, longSide))
	default:
		return ""
	}
}

// ResolveOpenAIImageSize maps resolution + aspect ratio to the same normalized size
// calculation used by the playground UI.
func ResolveOpenAIImageSize(resolution, aspectRatio string) (string, bool) {
	tier := normalizeOpenAIImageSizeTier(resolution)
	ar := normalizeOpenAIImageAspectRatio(aspectRatio)

	if ar == "auto" {
		return "auto", true
	}
	if tier == "" || ar == "" {
		return "auto", false
	}

	size := calculateOpenAIImageSize(tier, ar)
	if size == "" {
		return "auto", false
	}
	return size, true
}

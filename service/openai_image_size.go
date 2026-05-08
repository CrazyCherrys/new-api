package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

var openAIImageSizeMappings = map[string]map[string]string{
	"1K": {
		"1:1":  "1024x1024",
		"4:3":  "1024x1024",
		"3:4":  "1024x1024",
		"4:5":  "1024x1280",
		"5:4":  "1280x1024",
		"3:2":  "1536x1024",
		"16:9": "1536x1024",
		"21:9": "1536x1024",
		"2:3":  "1024x1536",
		"9:16": "1024x1536",
		"9:21": "1024x1536",
	},
	"2K": {
		"1:1":  "2048x2048",
		"4:3":  "2048x2048",
		"4:5":  "2048x2560",
		"5:4":  "2560x2048",
		"3:2":  "2048x1152",
		"16:9": "2048x1152",
		"21:9": "2048x1152",
		"2:3":  "2160x3840",
		"9:16": "2160x3840",
		"3:4":  "2160x3840",
		"9:21": "2160x3840",
	},
	"4K": {
		"1:1":  "2048x2048",
		"4:3":  "2048x2048",
		"3:4":  "2048x2048",
		"4:5":  "3072x3840",
		"5:4":  "3840x3072",
		"3:2":  "3840x2160",
		"16:9": "3840x2160",
		"21:9": "3840x2160",
		"2:3":  "2160x3840",
		"9:16": "2160x3840",
		"9:21": "2160x3840",
	},
}

func normalizeOpenAIImageResolution(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
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

// ResolveOpenAIImageSize maps an image resolution tier and aspect ratio to a concrete OpenAI size.
// It returns the resolved size string and whether the pair was explicitly understood.
//
// If the caller passes aspectRatio=auto, the size is intentionally auto-selected and returns true.
// If the pair is missing or unsupported, the function returns auto with false so callers can
// decide whether to keep auto or fall back to a legacy explicit size.
func ResolveOpenAIImageSize(resolution, aspectRatio string) (string, bool) {
	res := normalizeOpenAIImageResolution(resolution)
	ar := normalizeOpenAIImageAspectRatio(aspectRatio)

	if ar == "auto" {
		return "auto", true
	}
	if res == "" || ar == "" {
		return "auto", false
	}

	if sizeByRatio, ok := openAIImageSizeMappings[res]; ok {
		if size, ok := sizeByRatio[ar]; ok {
			return size, true
		}
	}

	return "auto", false
}

// AspectRatioDimensions represents width and height for a given aspect ratio
type AspectRatioDimensions struct {
	Width  int
	Height int
}

// supportedAspectRatios defines the standard aspect ratios and their base dimensions
// Based on gpt_image_playground reference implementation
var supportedAspectRatios = map[string]AspectRatioDimensions{
	"1:1":  {Width: 1024, Height: 1024},
	"16:9": {Width: 1792, Height: 1024},
	"9:16": {Width: 1024, Height: 1792},
	"4:3":  {Width: 1408, Height: 1024},
	"3:4":  {Width: 1024, Height: 1408},
}

// CalculateImageDimensions calculates width and height based on aspect ratio string
// Supports standard ratios (1:1, 16:9, 9:16, 4:3, 3:4) and custom ratios (e.g., "3:2")
// Returns width, height, and error if the ratio is invalid
func CalculateImageDimensions(aspectRatio string) (int, int, error) {
	normalized := normalizeOpenAIImageAspectRatio(aspectRatio)

	if normalized == "" || normalized == "auto" {
		// Default to 1:1 if not specified
		return 1024, 1024, nil
	}

	// Check if it's a standard supported ratio
	if dims, ok := supportedAspectRatios[normalized]; ok {
		return dims.Width, dims.Height, nil
	}

	// Parse custom ratio (e.g., "3:2", "21:9")
	parts := strings.Split(normalized, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid aspect ratio format: %s", aspectRatio)
	}

	widthRatio, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || widthRatio <= 0 {
		return 0, 0, fmt.Errorf("invalid width ratio: %s", parts[0])
	}

	heightRatio, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || heightRatio <= 0 {
		return 0, 0, fmt.Errorf("invalid height ratio: %s", parts[1])
	}

	// Calculate dimensions maintaining the ratio
	// Target total pixels around 1024*1024 = 1,048,576
	const targetPixels = 1024 * 1024
	ratio := widthRatio / heightRatio

	// Calculate height first, then width to maintain ratio
	height := math.Sqrt(float64(targetPixels) / ratio)
	width := height * ratio

	// Round to nearest multiple of 64 for better compatibility
	finalWidth := int(math.Round(width/64) * 64)
	finalHeight := int(math.Round(height/64) * 64)

	// Ensure minimum dimensions
	if finalWidth < 256 {
		finalWidth = 256
	}
	if finalHeight < 256 {
		finalHeight = 256
	}

	return finalWidth, finalHeight, nil
}

// FormatImageSize formats width and height into "WxH" string format
func FormatImageSize(width, height int) string {
	return fmt.Sprintf("%dx%d", width, height)
}

// ResolveImageSizeFromAspectRatio resolves aspect ratio to concrete size string
// This is a convenience function combining CalculateImageDimensions and FormatImageSize
func ResolveImageSizeFromAspectRatio(aspectRatio string) (string, error) {
	width, height, err := CalculateImageDimensions(aspectRatio)
	if err != nil {
		return "", err
	}
	return FormatImageSize(width, height), nil
}

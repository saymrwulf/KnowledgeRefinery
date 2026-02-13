package extractors

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
	".heic": true, ".heif": true, ".tiff": true, ".tif": true,
}

// ImageExtractor uses macOS Vision OCR to extract text from images.
type ImageExtractor struct{}

func (e *ImageExtractor) Name() string     { return "image" }
func (e *ImageExtractor) Priority() int    { return 15 }

func (e *ImageExtractor) CanHandle(asset storage.FileAsset) bool {
	ext := strings.ToLower(filepath.Ext(asset.Filename))
	return imageExtensions[ext]
}

func (e *ImageExtractor) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	var atoms []storage.ContentAtom

	// Try OCR
	ocrText, err := visionOCR(asset.Path)
	if err != nil {
		slog.Warn("Vision OCR failed", "file", asset.Filename, "error", err)
	}

	seqIdx := 0
	if ocrText != "" {
		anchor := storage.EvidenceAnchor{AssetID: asset.ID}
		atom := storage.NewContentAtom(
			ComputeAtomID(asset.ID, storage.AtomText, seqIdx),
			asset.ID, storage.AtomText, seqIdx, anchor.ToJSON(),
		)
		atom.PayloadText = &ocrText
		atoms = append(atoms, atom)
		seqIdx++
	}

	// Image reference atom
	anchor := storage.EvidenceAnchor{AssetID: asset.ID}
	imgAtom := storage.NewContentAtom(
		ComputeAtomID(asset.ID, storage.AtomImage, seqIdx),
		asset.ID, storage.AtomImage, seqIdx, anchor.ToJSON(),
	)
	imgAtom.PayloadRef = &asset.Path
	atoms = append(atoms, imgAtom)

	return atoms, nil
}

// visionOCR runs macOS Vision framework via a Swift subprocess.
func visionOCR(imagePath string) (string, error) {
	swiftCode := fmt.Sprintf(`
import Foundation
import Vision
import AppKit

let url = URL(fileURLWithPath: "%s")
guard let image = NSImage(contentsOf: url),
      let tiffData = image.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiffData),
      let cgImage = bitmap.cgImage else {
    exit(1)
}

let request = VNRecognizeTextRequest()
request.recognitionLevel = .accurate
request.usesLanguageCorrection = true

let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
try handler.perform([request])

guard let observations = request.results else { exit(0) }
for observation in observations {
    if let candidate = observation.topCandidates(1).first {
        print(candidate.string)
    }
}
`, imagePath)

	cmd := exec.Command("swift", "-")
	cmd.Stdin = strings.NewReader(swiftCode)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

package extractors

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// DICOMExtractor does minimal binary header parsing for DICOM files.
type DICOMExtractor struct{}

func (e *DICOMExtractor) Name() string     { return "dicom" }
func (e *DICOMExtractor) Priority() int    { return 15 }

func (e *DICOMExtractor) CanHandle(asset storage.FileAsset) bool {
	ext := strings.ToLower(filepath.Ext(asset.Filename))
	return ext == ".dcm" || ext == ".dicom"
}

func (e *DICOMExtractor) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	data, err := os.ReadFile(asset.Path)
	if err != nil {
		return nil, err
	}

	metadata := parseDICOMMetadata(data)

	var atoms []storage.ContentAtom
	seqIdx := 0

	// Text summary
	var parts []string
	if v, ok := metadata["PatientName"]; ok {
		parts = append(parts, fmt.Sprintf("Patient: %s", v))
	}
	if v, ok := metadata["StudyDescription"]; ok {
		parts = append(parts, fmt.Sprintf("Study: %s", v))
	}
	if v, ok := metadata["Modality"]; ok {
		parts = append(parts, fmt.Sprintf("Modality: %s", v))
	}
	if v, ok := metadata["Manufacturer"]; ok {
		parts = append(parts, fmt.Sprintf("Manufacturer: %s", v))
	}

	if len(parts) > 0 {
		text := strings.Join(parts, "\n")
		anchor := storage.EvidenceAnchor{AssetID: asset.ID}
		atom := storage.NewContentAtom(
			ComputeAtomID(asset.ID, storage.AtomText, seqIdx),
			asset.ID, storage.AtomText, seqIdx, anchor.ToJSON(),
		)
		atom.PayloadText = &text
		atoms = append(atoms, atom)
		seqIdx++
	}

	// Metadata atom
	if len(metadata) > 0 {
		metaJSON, _ := json.Marshal(metadata)
		metaStr := string(metaJSON)
		anchor := storage.EvidenceAnchor{AssetID: asset.ID}
		atom := storage.NewContentAtom(
			ComputeAtomID(asset.ID, storage.AtomMetadata, seqIdx),
			asset.ID, storage.AtomMetadata, seqIdx, anchor.ToJSON(),
		)
		atom.MetadataJSON = &metaStr
		atoms = append(atoms, atom)
		seqIdx++
	}

	// Image reference
	anchor := storage.EvidenceAnchor{AssetID: asset.ID}
	imgAtom := storage.NewContentAtom(
		ComputeAtomID(asset.ID, storage.AtomImage, seqIdx),
		asset.ID, storage.AtomImage, seqIdx, anchor.ToJSON(),
	)
	imgAtom.PayloadRef = &asset.Path
	atoms = append(atoms, imgAtom)

	return atoms, nil
}

// parseDICOMMetadata does minimal DICOM binary parsing.
func parseDICOMMetadata(data []byte) map[string]string {
	meta := make(map[string]string)

	// Check DICM magic at offset 128
	if len(data) < 136 {
		return meta
	}
	if string(data[128:132]) != "DICM" {
		return meta
	}

	// Known DICOM tags (group, element) -> name
	tags := map[[2]uint16]string{
		{0x0010, 0x0010}: "PatientName",
		{0x0010, 0x0020}: "PatientID",
		{0x0008, 0x1030}: "StudyDescription",
		{0x0008, 0x103E}: "SeriesDescription",
		{0x0008, 0x0060}: "Modality",
		{0x0008, 0x0070}: "Manufacturer",
		{0x0008, 0x0080}: "InstitutionName",
		{0x0008, 0x0020}: "StudyDate",
	}

	offset := 132
	for offset+8 <= len(data) {
		if offset > 10000 { // Don't scan too far
			break
		}
		group := binary.LittleEndian.Uint16(data[offset:])
		element := binary.LittleEndian.Uint16(data[offset+2:])
		key := [2]uint16{group, element}

		// Skip meta-header group (0002) with explicit VR
		vr := string(data[offset+4 : offset+6])
		var length uint32
		isExplicitLong := vr == "OB" || vr == "OW" || vr == "OF" || vr == "SQ" || vr == "UC" || vr == "UN" || vr == "UR" || vr == "UT"

		if isExplicitLong {
			if offset+12 > len(data) {
				break
			}
			length = binary.LittleEndian.Uint32(data[offset+8:])
			offset += 12
		} else if vr[0] >= 'A' && vr[0] <= 'Z' && vr[1] >= 'A' && vr[1] <= 'Z' {
			// Explicit VR, short
			if offset+8 > len(data) {
				break
			}
			length = uint32(binary.LittleEndian.Uint16(data[offset+6:]))
			offset += 8
		} else {
			// Implicit VR
			if offset+8 > len(data) {
				break
			}
			length = binary.LittleEndian.Uint32(data[offset+4:])
			offset += 8
		}

		if length == 0xFFFFFFFF || length > 10000 {
			break
		}

		if offset+int(length) > len(data) {
			break
		}

		if name, ok := tags[key]; ok {
			value := strings.TrimRight(string(data[offset:offset+int(length)]), "\x00 ")
			if value != "" {
				meta[name] = value
			}
		}

		offset += int(length)
	}

	return meta
}

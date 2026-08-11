package models

import "vda5050/common"

// Minimal visualization message — just enough to drive the viewer's
// moving marker. The full agvPosition shape (localizationScore,
// deviationRange, ...) is Session 5/8 content.
type Visualization struct {
	common.Header
	AgvPosition struct {
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		Theta float64 `json:"theta"`
		MapId string  `json:"mapId"`
	} `json:"agvPosition"`
}

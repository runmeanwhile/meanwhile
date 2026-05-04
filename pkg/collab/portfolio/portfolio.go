package portfolio

import (
	"sort"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/collab/evidencegate"
)

// BetType classifies risk posture for a concept.
type BetType string

const (
	BetSafe     BetType = "safe"
	BetAdjacent BetType = "adjacent"
	BetBold     BetType = "bold"
)

// Bet maps one evidence card into a portfolio slot.
type Bet struct {
	Type BetType           `json:"type"`
	Card evidencegate.Card `json:"card"`
}

// Classify maps a card to a bet type using risk labels and lightweight heuristics.
func Classify(card evidencegate.Card) BetType {
	risk := strings.ToLower(strings.TrimSpace(card.RiskLevel))
	confidence := strings.ToLower(strings.TrimSpace(card.Confidence))
	switch {
	case strings.Contains(risk, "low"), strings.Contains(risk, "safe"):
		return BetSafe
	case strings.Contains(risk, "high"), strings.Contains(risk, "bold"):
		return BetBold
	case strings.Contains(risk, "medium"):
		return BetAdjacent
	}

	switch {
	case strings.Contains(confidence, "low"):
		return BetBold
	case strings.Contains(confidence, "high"):
		return BetSafe
	default:
		return BetAdjacent
	}
}

// Build selects a diverse portfolio from cards.
func Build(cards []evidencegate.Card, limit int) []Bet {
	if len(cards) == 0 || limit <= 0 {
		return nil
	}

	type scored struct {
		card  evidencegate.Card
		score int
	}

	buckets := map[BetType][]scored{
		BetSafe:     nil,
		BetAdjacent: nil,
		BetBold:     nil,
	}
	all := make([]scored, 0, len(cards))
	for _, card := range cards {
		item := scored{card: card, score: evidencegate.ScoreCard(card)}
		kind := Classify(card)
		buckets[kind] = append(buckets[kind], item)
		all = append(all, item)
	}

	sortByScore := func(items []scored) {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].score == items[j].score {
				return items[i].card.Title < items[j].card.Title
			}
			return items[i].score > items[j].score
		})
	}
	for kind, items := range buckets {
		sortByScore(items)
		buckets[kind] = items
	}
	sortByScore(all)

	out := make([]Bet, 0, limit)
	used := make(map[string]struct{})
	add := func(kind BetType, item scored) {
		key := strings.TrimSpace(item.card.Title + "::" + item.card.Concept)
		if key == "" {
			key = item.card.Title
		}
		if _, ok := used[key]; ok {
			return
		}
		used[key] = struct{}{}
		out = append(out, Bet{Type: kind, Card: item.card})
	}

	seedOrder := []BetType{BetSafe, BetAdjacent, BetBold}
	for _, kind := range seedOrder {
		if len(out) >= limit {
			break
		}
		items := buckets[kind]
		if len(items) == 0 {
			continue
		}
		add(kind, items[0])
	}

	if len(out) >= limit {
		return out[:limit]
	}
	for _, item := range all {
		if len(out) >= limit {
			break
		}
		add(Classify(item.card), item)
	}

	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

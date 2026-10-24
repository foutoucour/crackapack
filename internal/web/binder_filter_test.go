package web

import "testing"

func TestBinderFilterKeepsSlot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		show string
		slot slotView
		want bool
	}{
		{name: "no show filter keeps an empty slot", slot: slotView{}, want: true},
		{name: "duplicates drop a playset", show: showDuplicates, slot: slotView{Regular: 4}},
		{name: "duplicates keep a card above a playset", show: showDuplicates, slot: slotView{Regular: 5}, want: true},
		{name: "duplicates add the finishes of a card", show: showDuplicates, slot: slotView{Regular: 3, Foil: 2}, want: true},
		{name: "duplicates drop an empty slot", show: showDuplicates, slot: slotView{}},
		{name: "counter keeps a card that can be bought", show: showCounter, slot: slotView{Buy: buyView{Buyable: true}}, want: true},
		{name: "counter drops a card out of stock", show: showCounter, slot: slotView{Buy: buyView{Note: "Out of stock, never seen at the Counter"}}},
		{name: "trade keeps a card an NPC holds", show: showTrade, slot: slotView{Hunt: &huntView{OnShelf: true}}, want: true},
		{name: "trade drops a card nobody holds", show: showTrade, slot: slotView{Hunt: &huntView{}}},
		{name: "trade drops a slot with no hunt", show: showTrade, slot: slotView{}},
		{name: "an unknown choice keeps nothing", show: "nonsense", slot: slotView{Regular: 9, Buy: buyView{Buyable: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := (binderFilter{Show: tt.show}).keepsSlot(tt.slot); got != tt.want {
				t.Errorf("keepsSlot = %v, want %v", got, tt.want)
			}
		})
	}
}

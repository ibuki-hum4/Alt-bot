package casino

import "testing"

func TestBlackjackCardRank(t *testing.T) {
	cases := []struct {
		value int
		want  int
	}{
		{0, 0},   // Ace of first suit
		{12, 12}, // King of first suit
		{13, 0},  // Ace of second suit
		{25, 12}, // King of second suit
	}
	for _, c := range cases {
		if got := blackjackCardRank(blackjackCard{Value: c.value}); got != c.want {
			t.Errorf("blackjackCardRank(%d) = %d, want %d", c.value, got, c.want)
		}
	}
}

func TestBlackjackCardValue(t *testing.T) {
	cases := []struct {
		rank int
		want int
	}{
		{0, blackjackAceValue}, // Ace
		{9, 10},                // 10
		{10, 10},               // Jack
		{11, 10},               // Queen
		{12, 10},               // King
		{5, 6},                 // 6
	}
	for _, c := range cases {
		if got := blackjackCardValue(blackjackCard{Value: c.rank}); got != c.want {
			t.Errorf("blackjackCardValue(rank=%d) = %d, want %d", c.rank, got, c.want)
		}
	}
}

func TestBlackjackHandValue(t *testing.T) {
	cases := []struct {
		name      string
		ranks     []int
		wantTotal int
		wantSoft  bool
	}{
		{"hard_20", []int{9, 9}, 20, false},               // 10 + 10
		{"soft_blackjack", []int{0, 12}, 21, true},         // Ace + King -> soft 21
		{"ace_busts_to_hard", []int{0, 9, 9}, 21, false},   // A+10+10 = 21 hard (ace counted as 1)
		{"two_aces", []int{0, 0}, 12, true},                // A+A = 12 soft
		{"bust", []int{12, 11, 9}, 30, false},              // K+Q+10 = 30 bust
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cards := make([]blackjackCard, len(c.ranks))
			for i, r := range c.ranks {
				cards[i] = blackjackCard{Value: r}
			}
			total, soft := blackjackHandValue(cards)
			if total != c.wantTotal || soft != c.wantSoft {
				t.Errorf("blackjackHandValue(%v) = (%d, %v), want (%d, %v)", c.ranks, total, soft, c.wantTotal, c.wantSoft)
			}
		})
	}
}

func TestBlackjackResolvePayoutDealerBust(t *testing.T) {
	session := &blackjackSession{
		Dealer: []blackjackCard{{Value: 12}, {Value: 11}, {Value: 9}}, // K+Q+10 = 30 bust
		Hands: []blackjackHand{
			{Cards: []blackjackCard{{Value: 9}, {Value: 8}}, Bet: 100}, // 10+9=19, not busted
		},
	}
	if got := blackjackResolvePayout(session); got != 200 {
		t.Errorf("blackjackResolvePayout (dealer bust) = %d, want 200 (2x bet)", got)
	}
}

func TestBlackjackResolvePayoutPush(t *testing.T) {
	session := &blackjackSession{
		Dealer: []blackjackCard{{Value: 9}, {Value: 8}}, // 19
		Hands: []blackjackHand{
			{Cards: []blackjackCard{{Value: 9}, {Value: 8}}, Bet: 100}, // 19, push
		},
	}
	if got := blackjackResolvePayout(session); got != 100 {
		t.Errorf("blackjackResolvePayout (push) = %d, want 100 (bet returned)", got)
	}
}

func TestBlackjackResolvePayoutPlayerLoses(t *testing.T) {
	session := &blackjackSession{
		Dealer: []blackjackCard{{Value: 12}, {Value: 8}}, // K+9 = 19
		Hands: []blackjackHand{
			{Cards: []blackjackCard{{Value: 7}, {Value: 6}}, Bet: 100}, // 8+7=15, loses
		},
	}
	if got := blackjackResolvePayout(session); got != 0 {
		t.Errorf("blackjackResolvePayout (loss) = %d, want 0", got)
	}
}

func TestBlackjackResolvePayoutNaturalBlackjack(t *testing.T) {
	session := &blackjackSession{
		Dealer: []blackjackCard{{Value: 9}, {Value: 8}}, // 19, no natural
		Hands: []blackjackHand{
			{Cards: []blackjackCard{{Value: 0}, {Value: 12}}, Bet: 100, Natural: true}, // A+K = 21 natural
		},
	}
	// payout = bet + bet + bet/2 = 100 + 100 + 50 = 250 (3:2 blackjack payout)
	if got := blackjackResolvePayout(session); got != 250 {
		t.Errorf("blackjackResolvePayout (natural blackjack) = %d, want 250", got)
	}
}

func TestBlackjackResolvePayoutBustedHandExcluded(t *testing.T) {
	session := &blackjackSession{
		Dealer: []blackjackCard{{Value: 9}, {Value: 8}}, // 19
		Hands: []blackjackHand{
			{Cards: []blackjackCard{{Value: 12}, {Value: 11}, {Value: 9}}, Bet: 100, Busted: true}, // 30, busted
		},
	}
	if got := blackjackResolvePayout(session); got != 0 {
		t.Errorf("blackjackResolvePayout (busted hand) = %d, want 0", got)
	}
}

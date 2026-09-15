package scraper

import "testing"

func TestIsPromotionalOrOmnibusVariant(t *testing.T) {
	testCases := []struct {
		id       string
		title    string
		orig     string
		expected bool
	}{
		{"1DLDSS559TK", "Title", "【数量限定】... パンティと写真付き", true},
		{"1DLDSS545TK", "Title", "【数量限定】... パンティと写真付き", true},
		{"MKMP-765", "中出し懇願 白濁マ○コBEST 淫乱30人種付け240分", "中出し懇願 白濁マ○コBEST 淫乱30人種付け240分", true},
		{"MIZD-550", "Hinako Mori: MOODYZ Exclusive 1st BEST", "森日向子 MOODYZ専属1stBEST", true},
		{"9OFJE-548", "Compilation", "オムニバス", true},
		{"1SETH00008", "Compilation", "オムニバス", true},
		{"TKCJOD-510", "Title", "チェキ付き", true},
		{"FSDSS-559TK1", "Title", "生写真付き", true},
		// Genuine releases should NOT be skipped
		{"DLDSS-559", "Teacher Romance", "’多分、娘が好きであろう’娘の家庭●師と週3日、自宅不貞行為に及んでいますー。 角奈保", false},
		{"DLDSS-545", "Teacher Romance", "不良生徒から優等生を守るため…人妻女●師 放課後肉便器 角奈保/花守夏歩", false},
		{"SNOS-038", "Sample Movie", "Sample", false},
		{"SONE-682", "Potential Eros Explosion Of The Strongest Heroine Kanna Seto", "最強ヒロインの潜在的エロス爆発 瀬戸環奈", false},
	}

	for _, tc := range testCases {
		skip, reason := IsPromotionalOrOmnibusVariant(tc.id, tc.title, tc.orig, "", nil)
		if skip != tc.expected {
			t.Errorf("IsPromotionalOrOmnibusVariant(%s) = (%v, %s); expected skip = %v", tc.id, skip, reason, tc.expected)
		}
	}
}

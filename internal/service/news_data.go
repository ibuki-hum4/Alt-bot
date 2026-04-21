package service

import "fmt"

type EventType string

const (
	EventCrash          EventType = "CRASH"
	EventMoon           EventType = "MOON"
	EventHoliday        EventType = "HOLIDAY"
	EventStagnation     EventType = "STAGNATION"
	EventWhaleAlert     EventType = "WHALE_ALERT"
	EventBurnEvent      EventType = "BURN_EVENT"
	EventFOMO           EventType = "FOMO"
	EventInsiderLeak    EventType = "INSIDER_LEAK"
	EventBubble         EventType = "BUBBLE"
	EventShortSqueeze   EventType = "SHORT_SQUEEZE"
	EventRegulation     EventType = "REGULATION"
	EventDeflationShock EventType = "DEFLATION_SHOCK"
	EventGoldenCross    EventType = "GOLDEN_CROSS"
)

type NewsStory struct {
	Title       string
	Description string
	Color       int
	ImpactLevel string
}

var NewsStories = map[EventType]NewsStory{
	EventCrash: {
		Title:       "【緊急】ALToken大暴落",
		Description: "大手取引所のウォレットから大量のALTが不正流出したとの噂が拡散。投資家たちがパニック売りに走っています！",
		Color:       0xE74C3C,
		ImpactLevel: "激甚",
	},
	EventMoon: {
		Title:       "【速報】月まで飛べ！",
		Description: "世界的EVメーカーのCEOが『今後はALTokenを公式決済に採用する』と発表。買い注文が殺到し、月を突き抜ける勢いです！",
		Color:       0xF1C40F,
		ImpactLevel: "激甚",
	},
	EventHoliday: {
		Title:       "市場休場（リラックス・デー）",
		Description: "今日はALToken財団の創立記念日。トレーダーたちも休暇に入り、値動きは穏やかな小春日和となっています。",
		Color:       0x2ECC71,
		ImpactLevel: "マイルド",
	},
	EventStagnation: {
		Title:       "凪（なぎ）の相場",
		Description: "新たな材料待ちの状態です。オーダーブックは静まり返り、1円を巡る静かな攻防が続いています。",
		Color:       0x95A5A6,
		ImpactLevel: "マイルド",
	},
	EventWhaleAlert: {
		Title:       "クジラ浮上観測",
		Description: "深海から巨大なクジラ（大富豪）が姿を現しました。{{.Amount}}Yen規模の取引が市場の波形を歪めています！",
		Color:       0x3498DB,
		ImpactLevel: "注意",
	},
	EventBurnEvent: {
		Title:       "デフレの炎",
		Description: "運営が手数料として回収したYenを焼却炉に投下しました。市場に流通する通貨が絞られ、価値の裏付けが強化されます。",
		Color:       0xE67E22,
		ImpactLevel: "注意",
	},
	EventFOMO: {
		Title:       "乗り遅れるな！熱狂の渦",
		Description: "SNSでトレンド入り！『今買わないと一生後悔する』という焦燥感が広がり、スリッページを無視した買いが続いています。",
		Color:       0x9B59B6,
		ImpactLevel: "激甚",
	},
	EventInsiderLeak: {
		Title:       "闇サイトの密告",
		Description: "ダークウェブで次回の政策決定に関する内部資料が流出しました。一部の鋭い投資家たちが密かに動き始めています…",
		Color:       0x34495E,
		ImpactLevel: "注意",
	},
	EventBubble: {
		Title:       "バブル経済・狂騒曲",
		Description: "実態を無視した価格上昇が続いています。誰もが自分だけは逃げ切れると信じ、最後の一人がババを引くのを待っています。",
		Color:       0xFF66CC,
		ImpactLevel: "激甚",
	},
	EventShortSqueeze: {
		Title:       "空売り勢の悲鳴",
		Description: "安値で買い戻そうとした投資家たちが、急激な反発に耐えきれず強制決済。売りが買いを呼ぶ地獄の連鎖が始まっています！",
		Color:       0xE84393,
		ImpactLevel: "激甚",
	},
	EventRegulation: {
		Title:       "当局の鉄槌",
		Description: "金融監視当局が仮想通貨の不正利用を警戒し、取引規制を発表。手続きの複雑化により取引コストが跳ね上がっています。",
		Color:       0xC0392B,
		ImpactLevel: "注意",
	},
	EventDeflationShock: {
		Title:       "預金封鎖の衝撃",
		Description: "経済の健全化を名目に、全ユーザーの口座から一律で1%の特別徴収が行われました。Yenの希少価値を高める苦肉の策です。",
		Color:       0x8E44AD,
		ImpactLevel: "注意",
	},
	EventGoldenCross: {
		Title:       "黄金の羅針盤",
		Description: "テクニカル指標が歴史的な強気サインを点灯。今、仕事に励んで資金を作れば、大きなリターンが期待できるでしょう。",
		Color:       0xF39C12,
		ImpactLevel: "マイルド",
	},
}

func ParseEventType(raw string) (EventType, error) {
	e := EventType(raw)
	if _, ok := NewsStories[e]; ok {
		return e, nil
	}
	return "", fmt.Errorf("unknown event type: %s", raw)
}

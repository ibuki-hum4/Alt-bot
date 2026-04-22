package commands

import (
	"github.com/disgoorg/disgo/discord"
	disgojson "github.com/disgoorg/json"
)

func Definitions() []discord.ApplicationCommandCreate {
	return []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "ping",
			Description: "Botの応答速度を埋め込みで返します",
		},
		discord.SlashCommandCreate{
			Name:        "help",
			Description: "利用できるコマンド一覧を表示",
		},
		discord.SlashCommandCreate{
			Name:        "work",
			Description: "仕事でYenを稼ぎます(ボタン選択)",
		},
		discord.SlashCommandCreate{
			Name:        "crypto",
			Description: "ALTokenを売買します",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "buy",
					Description: "YenでALTokenを購入",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{
							Name:        "amount",
							Description: "購入枚数",
							Required:    true,
							MinValue:    intPointer(1),
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "sell",
					Description: "ALTokenを売却してYenを受け取る",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{
							Name:        "amount",
							Description: "売却枚数",
							Required:    true,
							MinValue:    intPointer(1),
						},
					},
				},
			},
		},
		discord.SlashCommandCreate{
			Name:        "casino",
			Description: "カジノゲームを実行",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{Name: "blackjack", Description: "ブラックジャック"},
				discord.ApplicationCommandOptionSubCommand{Name: "chinchiro", Description: "チンチロ"},
				discord.ApplicationCommandOptionSubCommand{Name: "roulette", Description: "ルーレット"},
				discord.ApplicationCommandOptionSubCommand{Name: "slot", Description: "スロット"},
				discord.ApplicationCommandOptionSubCommand{Name: "poker", Description: "ポーカー"},
			},
		},
		discord.SlashCommandCreate{
			Name:                     "commands",
			Description:              "Botコマンド管理",
			DefaultMemberPermissions: disgojson.NewNullablePtr(discord.PermissionManageGuild),
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "reload",
					Description: "コマンドを再登録します(Owner限定)",
				},
			},
		},
		discord.SlashCommandCreate{
			Name:        "news",
			Description: "ニュース自動配信チャンネルを設定",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "channel",
					Description: "自動配信先チャンネルを設定",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionChannel{
							Name:        "channel",
							Description: "自動配信先チャンネル",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "off",
					Description: "ニュース自動配信を停止",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "status",
					Description: "ニュース自動配信の現在設定を表示",
				},
			},
		},
		discord.SlashCommandCreate{
			Name:                     "mod",
			Description:              "モデレーションユーティリティ",
			DefaultMemberPermissions: disgojson.NewNullablePtr(discord.PermissionManageGuild),
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "kick",
					Description: "ユーザーをキック",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "対象ユーザー",
							Required:    true,
						},
						discord.ApplicationCommandOptionString{
							Name:        "reason",
							Description: "理由(任意)",
							Required:    false,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "ban",
					Description: "ユーザーをBAN",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "対象ユーザー",
							Required:    true,
						},
						discord.ApplicationCommandOptionInt{
							Name:        "delete_days",
							Description: "削除する過去メッセージ日数(0-7)",
							Required:    false,
							MinValue:    intPointer(0),
							MaxValue:    intPointer(7),
						},
						discord.ApplicationCommandOptionString{
							Name:        "reason",
							Description: "理由(任意)",
							Required:    false,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "mute",
					Description: "ユーザーをタイムアウト",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionUser{
							Name:        "user",
							Description: "対象ユーザー",
							Required:    true,
						},
						discord.ApplicationCommandOptionInt{
							Name:        "minutes",
							Description: "ミュート時間(1-10080分)",
							Required:    true,
							MinValue:    intPointer(1),
							MaxValue:    intPointer(10080),
						},
						discord.ApplicationCommandOptionString{
							Name:        "reason",
							Description: "理由(任意)",
							Required:    false,
						},
					},
				},
			},
		},
		discord.SlashCommandCreate{
			Name:        "rate",
			Description: "現在価格と24h変動を表示",
		},
		discord.SlashCommandCreate{
			Name:        "chart",
			Description: "価格履歴を表示",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionInt{
					Name:        "limit",
					Description: "表示件数(1-50, 既定20)",
					Required:    false,
					MinValue:    intPointer(1),
					MaxValue:    intPointer(50),
				},
			},
		},
	}
}

func intPointer(v int) *int {
	return &v
}

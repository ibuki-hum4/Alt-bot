package commands

import (
	"github.com/disgoorg/disgo/discord"
	disgojson "github.com/disgoorg/json"

	"alt-bot/internal/bot/rolepanel"
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
			Name:        "rob",
			Description: "他ユーザーからYenを強奪(失敗すると罰金、防犯カメラで防御可)",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionUser{
					Name:        "target",
					Description: "強奪対象ユーザー",
					Required:    true,
				},
			},
		},
		discord.SlashCommandCreate{
			Name:        "shop",
			Description: "ショップで商品を選択して購入(1分以内に操作)",
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
				discord.ApplicationCommandOptionSubCommand{
					Name:        "blackjack",
					Description: "ブラックジャック",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{Name: "amount", Description: "賭けるYen", Required: true, MinValue: intPointer(1)},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "chinchiro",
					Description: "チンチロ",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{Name: "amount", Description: "賭けるYen", Required: true, MinValue: intPointer(1)},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "mines",
					Description: "マインスイパー",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{
							Name:        "amount",
							Description: "賭けるYen",
							Required:    true,
							MinValue:    intPointer(1),
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "poker",
					Description: "簡易ポーカー",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionInt{
							Name:        "amount",
							Description: "賭けるYen",
							Required:    true,
							MinValue:    intPointer(1),
						},
					},
				},
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
			Name:                     "pin",
			Description:              "チャンネル最下部に固定表示するメッセージを設定",
			DefaultMemberPermissions: disgojson.NewNullablePtr(discord.PermissionManageGuild),
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "set",
					Description: "このチャンネルの固定メッセージを設定(入力欄が開きます)",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "off",
					Description: "このチャンネルの固定メッセージを解除",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "status",
					Description: "このチャンネルの固定メッセージ設定を表示",
				},
			},
		},
		discord.SlashCommandCreate{
			Name:                     "rp",
			Description:              "ロールパネル管理",
			DefaultMemberPermissions: disgojson.NewNullablePtr(discord.PermissionManageGuild),
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "create",
					Description: "ロールパネルを作成",
					Options: append(rolepanel.RoleOptions(),
						discord.ApplicationCommandOptionString{
							Name:        "title",
							Description: "埋め込みタイトル(省略時は既定)",
							Required:    false,
						},
						discord.ApplicationCommandOptionString{
							Name:        "description",
							Description: "埋め込み説明(省略時は既定)",
							Required:    false,
						},
					),
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "add",
					Description: "ロールパネルにロールを追加",
					Options: append([]discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:         "panel",
							Description:  "追加先ロールパネル",
							Required:     true,
							Autocomplete: true,
						},
					}, rolepanel.RoleOptions()...),
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "delete",
					Description: "ロールパネルを削除",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:         "panel",
							Description:  "削除するロールパネル",
							Required:     true,
							Autocomplete: true,
						},
					},
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

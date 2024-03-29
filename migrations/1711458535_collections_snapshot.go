package migrations

import (
	"encoding/json"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		jsonData := `[
			{
				"id": "_pb_users_auth_",
				"created": "2023-05-03 13:58:52.695Z",
				"updated": "2023-05-03 15:42:26.970Z",
				"name": "users",
				"type": "auth",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "users_name",
						"name": "name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [],
				"listRule": "id = @request.auth.id",
				"viewRule": "id = @request.auth.id",
				"createRule": "",
				"updateRule": "id = @request.auth.id",
				"deleteRule": "id = @request.auth.id",
				"options": {
					"allowEmailAuth": true,
					"allowOAuth2Auth": true,
					"allowUsernameAuth": true,
					"exceptEmailDomains": null,
					"manageRule": null,
					"minPasswordLength": 8,
					"onlyEmailDomains": null,
					"onlyVerified": false,
					"requireEmail": false
				}
			},
			{
				"id": "2z10pbo7szq8uls",
				"created": "2023-05-03 15:21:23.332Z",
				"updated": "2024-03-26 13:05:44.817Z",
				"name": "playing_levels",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "ttoom0cv",
						"name": "name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "fusz5bpd",
						"name": "index",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "pv9k4urq0k1cmlu",
				"created": "2023-05-03 15:21:56.473Z",
				"updated": "2024-03-26 13:04:40.091Z",
				"name": "clubs",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "fgvy8m6q",
						"name": "name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "qdgmvu6tof46nwd",
				"created": "2023-05-03 15:23:40.763Z",
				"updated": "2024-03-26 13:05:37.618Z",
				"name": "players",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "htf3qrnh",
						"name": "firstName",
						"type": "text",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "3nxbaf5d",
						"name": "lastName",
						"type": "text",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "1xaqf016",
						"name": "notes",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "wdqg1c2b",
						"name": "status",
						"type": "select",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSelect": 1,
							"values": [
								"notAttending",
								"attending",
								"injured",
								"forfeited",
								"disqualified"
							]
						}
					},
					{
						"system": false,
						"id": "3g6uchnu",
						"name": "club",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "pv9k4urq0k1cmlu",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "xq7lqom9imfoxlx",
				"created": "2023-05-03 15:25:26.590Z",
				"updated": "2024-03-26 13:04:49.936Z",
				"name": "competitions",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "bz0r6hux",
						"name": "teamSize",
						"type": "number",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "mzcn647e",
						"name": "genderCategory",
						"type": "select",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSelect": 1,
							"values": [
								"female",
								"male",
								"mixed",
								"any"
							]
						}
					},
					{
						"system": false,
						"id": "sjdif5s5",
						"name": "ageGroup",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "25ej3p66dvflsof",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "vw8iepaf",
						"name": "playingLevel",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "2z10pbo7szq8uls",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "vcxdr13t",
						"name": "registrations",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "rvbyk7qe1ac5r5d",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "juj3jvkt",
						"name": "tournamentModeSettings",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "plqd8h814nig6ab",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "r3k3bgq3",
						"name": "seeds",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "rvbyk7qe1ac5r5d",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "a7bpvobq",
						"name": "draw",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "rvbyk7qe1ac5r5d",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "endvbim6",
						"name": "matches",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "acp4mh96d5gjfgn",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "vmj1xdzz",
						"name": "tieBreakers",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "n1ajffmo0whdfm4",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "rphgvzxq",
						"name": "rngSeed",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "vzoa6n7gmcfcwfn",
				"created": "2023-05-03 15:28:39.844Z",
				"updated": "2024-03-26 13:05:29.380Z",
				"name": "player_follows",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "d2pedulj",
						"name": "user",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "_pb_users_auth_",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "bvte5sjz",
						"name": "player",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "qdgmvu6tof46nwd",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "hdvkxtuf",
						"name": "subscriptions",
						"type": "select",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSelect": 2,
							"values": [
								"callOuts",
								"results"
							]
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "rvbyk7qe1ac5r5d",
				"created": "2023-05-03 15:30:23.644Z",
				"updated": "2024-03-26 13:05:51.684Z",
				"name": "teams",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "0wto0qlf",
						"name": "players",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "qdgmvu6tof46nwd",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "bb00m8cs",
						"name": "resigned",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "1lafq4a6mjxsbip",
				"created": "2023-05-03 15:35:42.814Z",
				"updated": "2024-03-26 13:05:06.077Z",
				"name": "gymnasiums",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "nfn99esk",
						"name": "name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "kn9y35ln",
						"name": "directions",
						"type": "editor",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"convertUrls": false
						}
					},
					{
						"system": false,
						"id": "zwjbzagg",
						"name": "rows",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "ywguxnvl",
						"name": "columns",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "1q6094tmbeif68j",
				"created": "2023-05-03 15:36:46.109Z",
				"updated": "2024-03-26 13:04:57.723Z",
				"name": "courts",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "xv2qlej5",
						"name": "gymnasium",
						"type": "relation",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "1lafq4a6mjxsbip",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "ykbe84ja",
						"name": "name",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "epekugmf",
						"name": "positionX",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "pfpczfml",
						"name": "positionY",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "ylhcxoir",
						"name": "isActive",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "acp4mh96d5gjfgn",
				"created": "2023-05-03 15:39:25.226Z",
				"updated": "2024-03-26 13:05:12.117Z",
				"name": "match_data",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "6lcaaass",
						"name": "sets",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "k2bk1bc253yap4e",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "98y5sqfa",
						"name": "court",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "1q6094tmbeif68j",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": 1,
							"displayFields": []
						}
					},
					{
						"system": false,
						"id": "v1wmkn9j",
						"name": "withdrawnTeams",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "rvbyk7qe1ac5r5d",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": null
						}
					},
					{
						"system": false,
						"id": "dwx5bqia",
						"name": "startTime",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "iiyyhrsm",
						"name": "courtAssignmentTime",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "jeh4sbka",
						"name": "endTime",
						"type": "date",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": "",
							"max": ""
						}
					},
					{
						"system": false,
						"id": "wrep7bmf",
						"name": "resultCard",
						"type": "file",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"mimeTypes": [],
							"thumbs": [],
							"maxSelect": 1,
							"maxSize": 5242880,
							"protected": false
						}
					},
					{
						"system": false,
						"id": "9wyx0weh",
						"name": "gameSheetPrinted",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "k2bk1bc253yap4e",
				"created": "2023-05-03 15:41:09.129Z",
				"updated": "2024-03-26 13:05:20.332Z",
				"name": "match_sets",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "aw1dtot7",
						"name": "team1Points",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "9ctgpkyk",
						"name": "team2Points",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "ezgnh50u37m74hc",
				"created": "2023-05-04 15:30:31.979Z",
				"updated": "2023-05-06 10:46:17.798Z",
				"name": "tournament_organizer",
				"type": "auth",
				"system": false,
				"schema": [],
				"indexes": [],
				"listRule": null,
				"viewRule": null,
				"createRule": "",
				"updateRule": null,
				"deleteRule": null,
				"options": {
					"allowEmailAuth": false,
					"allowOAuth2Auth": false,
					"allowUsernameAuth": true,
					"exceptEmailDomains": null,
					"manageRule": null,
					"minPasswordLength": 5,
					"onlyEmailDomains": null,
					"onlyVerified": false,
					"requireEmail": false
				}
			},
			{
				"id": "25ej3p66dvflsof",
				"created": "2023-05-24 17:11:25.945Z",
				"updated": "2024-03-26 13:04:22.463Z",
				"name": "age_groups",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "3simhxz8",
						"name": "age",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "rgspvmhi",
						"name": "type",
						"type": "select",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSelect": 1,
							"values": [
								"over",
								"under"
							]
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "ikvf45sbv9bccb2",
				"created": "2023-06-21 15:09:24.693Z",
				"updated": "2024-03-26 13:06:16.713Z",
				"name": "tournaments",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "0hxt4uz5",
						"name": "title",
						"type": "text",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"pattern": ""
						}
					},
					{
						"system": false,
						"id": "1vsznmxg",
						"name": "useAgeGroups",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					},
					{
						"system": false,
						"id": "bbkvrgd8",
						"name": "usePlayingLevels",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					},
					{
						"system": false,
						"id": "wj28qnti",
						"name": "dontReprintGameSheets",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					},
					{
						"system": false,
						"id": "7mei215l",
						"name": "printQrCodes",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					},
					{
						"system": false,
						"id": "4plm5f8q",
						"name": "playerRestTime",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "ea0rngxn",
						"name": "queueMode",
						"type": "select",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSelect": 1,
							"values": [
								"manual",
								"autoCourtAssignment",
								"auto"
							]
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "plqd8h814nig6ab",
				"created": "2023-09-03 16:16:19.153Z",
				"updated": "2024-03-26 13:06:07.728Z",
				"name": "tournament_mode_settings",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "vzwahdnn",
						"name": "type",
						"type": "select",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSelect": 1,
							"values": [
								"RoundRobin",
								"SingleElimination",
								"GroupKnockout",
								"DoubleElimination",
								"SingleEliminationWithConsolation"
							]
						}
					},
					{
						"system": false,
						"id": "exwn93lz",
						"name": "seedingMode",
						"type": "select",
						"required": true,
						"presentable": false,
						"unique": false,
						"options": {
							"maxSelect": 1,
							"values": [
								"random",
								"single",
								"tiered"
							]
						}
					},
					{
						"system": false,
						"id": "4agzgctg",
						"name": "passes",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "pyp8cf3i",
						"name": "numGroups",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "8vzehtdb",
						"name": "numQualifications",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "7bxj9ern",
						"name": "numConsolationRounds",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "9lxglqfe",
						"name": "placesToPlayOut",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "htkszfx3",
						"name": "winningPoints",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "mlwhvwny",
						"name": "winningSets",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "qjece1gs",
						"name": "maxPoints",
						"type": "number",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"min": null,
							"max": null,
							"noDecimal": false
						}
					},
					{
						"system": false,
						"id": "uruqy6s8",
						"name": "twoPointMargin",
						"type": "bool",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			},
			{
				"id": "n1ajffmo0whdfm4",
				"created": "2023-12-12 19:16:40.115Z",
				"updated": "2024-03-26 13:05:59.391Z",
				"name": "tie_breakers",
				"type": "base",
				"system": false,
				"schema": [
					{
						"system": false,
						"id": "ijfxliiy",
						"name": "tieBreakerRanking",
						"type": "relation",
						"required": false,
						"presentable": false,
						"unique": false,
						"options": {
							"collectionId": "rvbyk7qe1ac5r5d",
							"cascadeDelete": false,
							"minSelect": null,
							"maxSelect": null,
							"displayFields": null
						}
					}
				],
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"viewRule": "@request.auth.id != \"\"",
				"createRule": "@request.auth.id != \"\"",
				"updateRule": "@request.auth.id != \"\"",
				"deleteRule": "@request.auth.id != \"\"",
				"options": {}
			}
		]`

		collections := []*models.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collections); err != nil {
			return err
		}

		return daos.New(db).ImportCollections(collections, true, nil)
	}, func(db dbx.Builder) error {
		return nil
	})
}

package cmd

import (
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path"
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed data",
	Long:  `Seed some initial data`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Infof("Seed module")
		// Database
		var dialer gorm.Dialector
		if common.Config.Database.Dialer == "mysql" {
			dialer = mysql.Open(common.Config.Database.Uri)
		}else {
			var uri = path.Join(dir, os.Getenv("DATABASE_FOLDER"), "database.sqlite")
			if common.Config.Database.Uri != "" {
				uri = common.Config.Database.Uri
			}
			dialer = sqlite.Open(uri)
		}
		var err error
		common.Database, err = gorm.Open(dialer, &gorm.Config{})
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		common.Database.DB()
		common.Database.AutoMigrate(&models.Category{})
		common.Database.AutoMigrate(&models.Product{})
		common.Database.AutoMigrate(&models.Image{})
		//common.Database.AutoMigrate(&models.ProductProperty{})
		common.Database.AutoMigrate(&models.Variation{})
		common.Database.AutoMigrate(&models.Property{})
		common.Database.AutoMigrate(&models.Option{})
		common.Database.AutoMigrate(&models.Value{})
		common.Database.AutoMigrate(&models.Price{})
		// DEMO
		// Create category Living Areas
		livingAreasCategory := &models.Category{
			Name:  "living-areas",
			Title: "Living Areas",
		}
		if _, err := models.CreateCategory(common.Database, livingAreasCategory); err != nil {
			logger.Errorf("%v", err)
		}
		// Create category Living Areas >> Bathroom
		bathroomCategory := &models.Category{
			Name:   "bathroom",
			Title:  "Bathroom",
			Parent: livingAreasCategory,
		}
		if _, err := models.CreateCategory(common.Database, bathroomCategory); err == nil {
			logger.Errorf("%v", err)
		}
		// Create product #1
		// example: https://www.moebelhausduesseldorf.de/wohnraum/toilette/waschbeckenschrank-wei%c3%9f-f%c3%bcr-das-badezimmer-974057
		product1 := &models.Product{
			Name:        "washbasin-cabinet-white-for-the-bathroom",
			Title:       "Washbasin cabinet white for the bathroom",
			Description: "My description for humans and search engines engines may be here",
			Thumbnail: "https://shop.servhost.org/img/washbasin-cabinet-white-for-the-bathroom/washbasin-cabinet-white-for-the-bathroom-1.jpg",
		}
		if _, err := models.CreateProduct(common.Database, product1); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddProductToCategory(common.Database, bathroomCategory, product1); err != nil {
			logger.Errorf("%v", err)
		}
		//
		image1 := &models.Image{
			Url:    "https://shop.servhost.org/img/washbasin-cabinet-white-for-the-bathroom/washbasin-cabinet-white-for-the-bathroom-1.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, image1); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product1, image1); err != nil {
			logger.Errorf("%v", err)
		}
		image2 := &models.Image{
			Url:    "https://shop.servhost.org/img/washbasin-cabinet-white-for-the-bathroom/washbasin-cabinet-white-for-the-bathroom-2.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, image2); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product1, image2); err != nil {
			logger.Errorf("%v", err)
		}
		image3 := &models.Image{
			Url:    "https://shop.servhost.org/img/washbasin-cabinet-white-for-the-bathroom/washbasin-cabinet-white-for-the-bathroom-3.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, image3); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product1, image3); err != nil {
			logger.Errorf("%v", err)
		}
		//
		ral9010 := &models.Value{
			Title: "RAL9010 - Weiß lackiert - unsere beliebteste Farbe",
			Thumbnail: "https://shop.servhost.org/img/colors/ral9010.jpg",
			Value: "RAL9010",
		}
		ral9001 := &models.Value{
			Title: "RAL9001 - Leicht Cremeweiß lackiert",
			Thumbnail: "https://shop.servhost.org/img/colors/ral9001.jpg",
			Value: "RAL9001",
		}
		m803 := &models.Value{
			Title: "M803 - Pearl Grey Pinseleffekt",
			Thumbnail: "https://shop.servhost.org/img/colors/m803.jpg",
			Value: "M803",
		}
		p001 := &models.Value{
			Title: "P001 - Braun gewachst",
			Thumbnail: "https://shop.servhost.org/img/colors/p001.jpg",
			Value: "P001",
		}
		p002 := &models.Value{
			Title: "P002 - Braun gewachst",
			Thumbnail: "https://shop.servhost.org/img/colors/p002.jpg",
			Value: "P002",
		}
		p003 := &models.Value{
			Title: "P003 - Schwarz Lackiert",
			Thumbnail: "https://shop.servhost.org/img/colors/p003.jpg",
			Value: "P003",
		}
		// Option: color
		color := &models.Option{
			Name: "color",
			Title: "Color",
			Values: []*models.Value{
				ral9010,
				ral9001,
				m803,
				p001,
				p002,
				p003,
			},
		}
		if _, err := models.CreateOption(common.Database, color); err != nil {
			logger.Errorf("%v", err)
		}
		// Option: handles
		handle := &models.Option{
			Name: "handle",
			Title: "Handle",
			Values: []*models.Value{
				ral9010,
				ral9001,
				m803,
				p001,
				p002,
				p003,
			},
		}
		if _, err := models.CreateOption(common.Database, handle); err != nil {
			logger.Errorf("%v", err)
		}
		// Variation 1
		variation1 := &models.Variation{
			Name:       "with-pine-wood-top",
			Title:      "with Pine Wood Top",
			Thumbnail: "/img/pine-leaf.svg",
			Properties: []*models.Property{
				{
					Name: "body-color",
					Title: "Body Color",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: ral9010,
							Price: 1.23,
						},
						{
							Enabled: true,
							Value: ral9001,
							Price: 2.34,
						},
						{
							Enabled: true,
							Value: m803,
							Price: 3.45,
						},
					},
				},
				{
					Name: "plate-color",
					Title: "Plate Color",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: ral9010,
							Price: 3.21,
						},
						{
							Enabled: true,
							Value: ral9001,
							Price: 4.32,
						},
						{
							Enabled: true,
							Value: m803,
							Price: 5.43,
						},
					},
				},{
					Name: "drawer-handles",
					Title: "Drawer Handles",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: p001,
							Price: 1.01,
						},
						{
							Enabled: true,
							Value: p002,
							Price: 1.02,
						},
						{
							Enabled: true,
							Value: p003,
							Price: 1.03,
						},
					},
				},
				{
					Name: "door-handles",
					Title: "Door Handles",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: p001,
							Price: 1.11,
						},
						{
							Enabled: true,
							Value: p002,
							Price: 1.12,
						},
						{
							Enabled: true,
							Value: p003,
							Price: 1.13,
						},
					},
				},
			},
			BasePrice:      1000.0,
			ProductId:  product1.ID,
		}
		if _, err := models.CreateVariation(common.Database, variation1); err != nil {
			logger.Errorf("%v", err)
		}
		// Variation 2
		variation2 := &models.Variation{
			Name:       "with-oak-wood-top",
			Title:      "with Oak Wood Top",
			Thumbnail: "/img/oak-leaf.svg",
			Properties: []*models.Property{
				{
					Name: "body-color",
					Title: "Body Color",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: ral9010,
							Price: 1.23,
						},
						{
							Enabled: true,
							Value: ral9001,
							Price: 2.34,
						},
						{
							Enabled: true,
							Value: m803,
							Price: 3.45,
						},
					},
				},
				{
					Name: "plate-color",
					Title: "Plate Color",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: ral9010,
							Price: 3.21,
						},
						{
							Enabled: true,
							Value: ral9001,
							Price: 4.32,
						},
						{
							Enabled: true,
							Value: m803,
							Price: 5.43,
						},
					},
				},{
					Name: "drawer-handles",
					Title: "Drawer Handles",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: p001,
							Price: 1.01,
						},
						{
							Enabled: true,
							Value: p002,
							Price: 1.02,
						},
						{
							Enabled: true,
							Value: p003,
							Price: 1.03,
						},
					},
				},
				/*{
					Name: "door-handles",
					Title: "Door Handles",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: p001,
							Price: 1.11,
						},
						{
							Enabled: true,
							Value: p002,
							Price: 1.12,
						},
						{
							Enabled: true,
							Value: p003,
							Price: 1.13,
						},
					},
				},*/
			},
			BasePrice:      1200.0,
			ProductId:  product1.ID,
		}
		if _, err := models.CreateVariation(common.Database, variation2); err != nil {
			logger.Errorf("%v", err)
		}
		// Variation 3
		variation3 := &models.Variation{
			Name:       "round-washbasin",
			Title:      "Round Washbasin",
			BasePrice:      500.0,
			ProductId:  product1.ID,
		}
		if _, err := models.CreateVariation(common.Database, variation3); err != nil {
			logger.Errorf("%v", err)
		}
		// Create product #2
		// example: https://www.moebelhausduesseldorf.de/wohnraum/toilette/massivholz-rahmen-f%c3%bcr-einen-spiegel-landhaus-optik-970652
		product2 := &models.Product{
			Name:        "solid-wood-mirror-country-house-look",
			Title:       "Solid wood mirror country house look",
			Description: "Beautiful mirror",
			Thumbnail: "https://shop.servhost.org/img/solid-wood-mirror-country-house-look/solid-wood-mirror-country-house-look-1.jpg",
		}
		if _, err := models.CreateProduct(common.Database, product2); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddProductToCategory(common.Database, bathroomCategory, product2); err != nil {
			logger.Errorf("%v", err)
		}
		// Images
		product2Image1 := &models.Image{
			Url:    "https://shop.servhost.org/img/solid-wood-mirror-country-house-look/solid-wood-mirror-country-house-look-1.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, product2Image1); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product2, product2Image1); err != nil {
			logger.Errorf("%v", err)
		}
		product2Image2 := &models.Image{
			Url:    "https://shop.servhost.org/img/solid-wood-mirror-country-house-look/solid-wood-mirror-country-house-look-2.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, product2Image2); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product2, product2Image2); err != nil {
			logger.Errorf("%v", err)
		}
		// Value: Without Antique look
		withoutAntiqueLook := &models.Value{
			Title: "Without Antique Look",
			Thumbnail: "https://shop.servhost.org/img/antique/without-antique-look.jpg",
			Value: "withoutAntiqueLook",
		}
		// Value: Antique look
		withAntiqueLook := &models.Value{
			Title: "With Antique Look",
			Thumbnail: "https://shop.servhost.org/img/antique/with-antique-look.jpg",
			Value: "withAntiqueLook",
		}
		// Option: Antique look
		antiqueLookOption := &models.Option{
			Name: "antique-look",
			Title: "Antique look",
			Values: []*models.Value{
				withoutAntiqueLook,
				withAntiqueLook,
			},
		}
		if _, err := models.CreateOption(common.Database, antiqueLookOption); err != nil {
			logger.Errorf("%v", err)
		}
		// Variations
		// Variation 1
		product2Variation1 := &models.Variation{
			Name:       "default",
			Title:      "Default",
			Properties: []*models.Property{
				{
					Name: "body-color",
					Title: "Body Color",
					Option: color,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: ral9010,
							Price: 1.23,
						},
						{
							Enabled: true,
							Value: ral9001,
							Price: 2.34,
						},
						{
							Enabled: true,
							Value: m803,
							Price: 3.45,
						},
					},
				},
				{
					Name: "antique-look",
					Title: "Antique Look",
					Option: antiqueLookOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: withoutAntiqueLook,
							Price: 0,
						},
						{
							Enabled: true,
							Value: withAntiqueLook,
							Price: 12.34,
						},
					},
				},
			},
			BasePrice:      100.0,
			ProductId:  product2.ID,
		}
		if _, err := models.CreateVariation(common.Database, product2Variation1); err != nil {
			logger.Errorf("%v", err)
		}
		// Create category Living Areas >> Bathroom
		dinningRoomCategory := &models.Category{
			Name:   "dining-room",
			Title:  "Dining room",
			Parent: livingAreasCategory,
		}
		if _, err := models.CreateCategory(common.Database, dinningRoomCategory); err == nil {
			logger.Errorf("%v", err)
		}
		// Create product #3
		// example: https://www.moebelhausduesseldorf.de/wohnraum/esszimmer/esstisch-massiv-eiche-mit-freier-beinwahl-1013276
		product3 := &models.Product{
			Name:        "solid-dining-table-with-crossbones",
			Title:       "Solid dining table with crossbones",
			Description: "Strong table",
			Thumbnail: "https://shop.servhost.org/img/solid-dining-table-with-crossbones/solid-dining-table-with-crossbones-1.jpg",
		}
		if _, err := models.CreateProduct(common.Database, product3); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddProductToCategory(common.Database, dinningRoomCategory, product3); err != nil {
			logger.Errorf("%v", err)
		}
		// Images
		product3Image1 := &models.Image{
			Url:    "https://shop.servhost.org/img/solid-dining-table-with-crossbones/solid-dining-table-with-crossbones-1.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, product3Image1); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product3, product3Image1); err != nil {
			logger.Errorf("%v", err)
		}
		product3Image2 := &models.Image{
			Url:    "https://shop.servhost.org/img/solid-dining-table-with-crossbones/solid-dining-table-with-crossbones-2.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, product3Image2); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product3, product3Image2); err != nil {
			logger.Errorf("%v", err)
		}
		// Value: 78x120x100cm
		s78x120x100cm := &models.Value{
			Title: "78 x 120 x 100 cm",
			Value: "78x120x100cm",
		}
		// Value: 78x140x100cm
		s78x140x100cm := &models.Value{
			Title: "78 x 140 x 100 cm",
			Value: "78x140x100cm",
		}
		// Value: s78x160x100cm
		s78x160x100cm := &models.Value{
			Title: "78 x 160 x 100 cm",
			Value: "78x160x100cm",
		}
		// Option: Table Plate Size
		tablePlateSizeOption := &models.Option{
			Name: "table-plate-size",
			Title: "Plate Size",
			Values: []*models.Value{
				s78x120x100cm,
				s78x140x100cm,
				s78x160x100cm,
			},
		}
		if _, err := models.CreateOption(common.Database, tablePlateSizeOption); err != nil {
			logger.Errorf("%v", err)
		}
		// Value: Straight Edge
		straightEdge := &models.Value{
			Title: "Straight Edge",
			Thumbnail: "https://shop.servhost.org/img/edges/straight-edge.jpg",
			Value: "straight-edge",
		}
		// Value: Curved Edge
		curvedEdge := &models.Value{
			Title: "Curved Edge",
			Thumbnail: "https://shop.servhost.org/img/edges/curved-edge.jpg",
			Value: "curved-edge",
		}
		// Value: Trapezoid Edge
		trapezoidEdge := &models.Value{
			Title: "Trapezoid Edge",
			Thumbnail: "https://shop.servhost.org/img/edges/trapezoid-edge.jpg",
			Value: "trapezoid-edge",
		}
		// Option: Table Plate Size
		tablePlateEdgeOption := &models.Option{
			Name: "table-plate-edge",
			Title: "Plate Edge",
			Values: []*models.Value{
				straightEdge,
				curvedEdge,
				trapezoidEdge,
			},
		}
		if _, err := models.CreateOption(common.Database, tablePlateEdgeOption); err != nil {
			logger.Errorf("%v", err)
		}
		// Value: U Metal
		uMetal := &models.Value{
			Title: "U Metal",
			Thumbnail: "https://shop.servhost.org/img/legs/u-metal.jpg",
			Value: "u-metal",
		}
		// Value: x Metal
		xMetal := &models.Value{
			Title: "X Metal",
			Thumbnail: "https://shop.servhost.org/img/legs/x-metal.jpg",
			Value: "x-metal",
		}
		// Value: w Metal
		wMetal := &models.Value{
			Title: "W Metal",
			Thumbnail: "https://shop.servhost.org/img/legs/w-metal.jpg",
			Value: "w-metal",
		}
		// Option: Table Legs Size
		tableLegsOption := &models.Option{
			Name: "table-legs",
			Title: "Legs",
			Values: []*models.Value{
				uMetal,
				xMetal,
				wMetal,
			},
		}
		if _, err := models.CreateOption(common.Database, tableLegsOption); err != nil {
			logger.Errorf("%v", err)
		}
		// Value: Varnish
		varnish := &models.Value{
			Title: "Varnish",
			Value: "varnish",
		}
		// Value: Paint
		paint := &models.Value{
			Title: "Paint",
			Value: "paint",
		}
		// Option: Table Legs Size
		coatingOption := &models.Option{
			Name: "coating",
			Title: "Coating",
			Values: []*models.Value{
				varnish,
				paint,
			},
		}
		if _, err := models.CreateOption(common.Database, coatingOption); err != nil {
			logger.Errorf("%v", err)
		}
		// Variations
		// Variation 1
		product3Variation1 := &models.Variation{
			Name:       "default",
			Title:      "Default",
			Properties: []*models.Property{
				{
					Name: "table-plate-size",
					Title: "Plate Size",
					Option: tablePlateSizeOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: s78x120x100cm,
							Price: 150,
						},
						{
							Enabled: true,
							Value: s78x140x100cm,
							Price: 170,
						},
						{
							Enabled: true,
							Value: s78x160x100cm,
							Price: 185,
						},
					},
				},
				{
					Name: "table-plate-edge",
					Title: "Plate edge",
					Option: tablePlateEdgeOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: straightEdge,
							Price: 0,
						},
						{
							Enabled: true,
							Value: curvedEdge,
							Price: 10,
						},
						{
							Enabled: true,
							Value: trapezoidEdge,
							Price: 8,
						},
					},
				},
				{
					Name: "table-logs",
					Title: "Legs",
					Option: tableLegsOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: uMetal,
							Price: 0,
						},
						{
							Enabled: true,
							Value: xMetal,
							Price: 0,
						},
						{
							Enabled: true,
							Value: wMetal,
							Price: 30,
						},
					},
				},
			},
			BasePrice:      200.0,
			ProductId:  product3.ID,
		}
		if _, err := models.CreateVariation(common.Database, product3Variation1); err != nil {
			logger.Errorf("%v", err)
		}
		// Variation 2
		product3Variation2 := &models.Variation{
			Name:       "metal-frame",
			Title:      "Metal Frame",
			Properties: []*models.Property{
				{
					Name: "table-plate-size",
					Title: "Plate Size",
					Option: tablePlateSizeOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: s78x120x100cm,
							Price: 150,
						},
						{
							Enabled: true,
							Value: s78x140x100cm,
							Price: 170,
						},
						{
							Enabled: true,
							Value: s78x160x100cm,
							Price: 185,
						},
					},
				},
				{
					Name: "table-plate-edge",
					Title: "Plate edge",
					Option: tablePlateEdgeOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: straightEdge,
							Price: 0,
						},
						{
							Enabled: true,
							Value: curvedEdge,
							Price: 10,
						},
						{
							Enabled: true,
							Value: trapezoidEdge,
							Price: 8,
						},
					},
				},
				{
					Name: "table-logs",
					Title: "Legs",
					Option: tableLegsOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: uMetal,
							Price: 0,
						},
						{
							Enabled: true,
							Value: xMetal,
							Price: 0,
						},
						{
							Enabled: true,
							Value: wMetal,
							Price: 30,
						},
					},
				},
				{
					Name: "coating",
					Title: "Coating (painting)",
					Option: coatingOption,
					Prices: []*models.Price{
						{
							Enabled: true,
							Value: varnish,
							Price: 0,
						},
						{
							Enabled: true,
							Value: paint,
							Price: 25,
						},
					},
				},
			},
			BasePrice:      250.0,
			ProductId:  product3.ID,
		}
		if _, err := models.CreateVariation(common.Database, product3Variation2); err != nil {
			logger.Errorf("%v", err)
		}
		// Create product #4
		// example: https://www.moebelhausduesseldorf.de/wohnraum/toilette/rettungsring-mit-spiegel-970936
		product4 := &models.Product{
			Name:        "lifebuoy-with-mirror",
			Title:       "Lifebuoy with mirror",
			Description: "Funny mirror",
			Thumbnail: "https://shop.servhost.org/img/lifebuoy-with-mirror/lifebuoy-with-mirror-1.jpg",
		}
		if _, err := models.CreateProduct(common.Database, product4); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddProductToCategory(common.Database, bathroomCategory, product4); err != nil {
			logger.Errorf("%v", err)
		}
		// Images
		product4Image1 := &models.Image{
			Url:    "https://shop.servhost.org/img/lifebuoy-with-mirror/lifebuoy-with-mirror-1.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, product4Image1); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product4, product4Image1); err != nil {
			logger.Errorf("%v", err)
		}
		// Variation 1
		product4Variation1 := &models.Variation{
			Name:       "default",
			Title:      "Default",
			Thumbnail: "https://shop.servhost.org/img/lifebuoy-with-mirror/lifebuoy-with-mirror-1.jpg",
			BasePrice:  75.0,
			ProductId:  product4.ID,
		}
		if _, err := models.CreateVariation(common.Database, product4Variation1); err != nil {
			logger.Errorf("%v", err)
		}

		// Create product #5
		// example: https://www.moebelhausduesseldorf.de/wohnraum/landhaus-buffetschrank-958300
		product5 := &models.Product{
			Name:        "country-house-buffet-cabinet",
			Title:       "Country house buffet cabinet",
			Description: "Funny mirror",
			Thumbnail: "https://shop.servhost.org/img/country-house-buffet-cabinet/country-house-buffet-cabinet-1.jpg",
		}
		if _, err := models.CreateProduct(common.Database, product5); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddProductToCategory(common.Database, livingAreasCategory, product5); err != nil {
			logger.Errorf("%v", err)
		}
		// Images
		product5Image1 := &models.Image{
			Url:    "https://shop.servhost.org/img/country-house-buffet-cabinet/country-house-buffet-cabinet-1.jpg",
			Width:  453,
			Height: 453,
		}
		if _, err := models.CreateImage(common.Database, product5Image1); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddImageToProduct(common.Database, product5, product5Image1); err != nil {
			logger.Errorf("%v", err)
		}
		// Variation 1
		product5Variation1 := &models.Variation{
			Name:       "default",
			Title:      "Default",
			Thumbnail: "https://shop.servhost.org/img/country-house-buffet-cabinet/country-house-buffet-cabinet-1.jpg",
			BasePrice:  1750.0,
			ProductId:  product5.ID,
		}
		if _, err := models.CreateVariation(common.Database, product5Variation1); err != nil {
			logger.Errorf("%v", err)
		}
		// /DEMO
	},
}

func init() {
	RootCmd.AddCommand(seedCmd)
}
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

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug stuff",
	Long:  `Debug stuff`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Infof("Debug module")
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
		/*common.Database.Migrator().DropTable(&models.Order{})
		common.Database.Migrator().DropTable(&models.Item{})*/
		if users, err := models.GetUsers(common.Database); err == nil {
			for _, user := range users {
				if user.ID > 1 {
					user.Role = models.ROLE_USER
					common.Database.Save(&user)
				}
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(debugCmd)
	//renderCmd.Flags().StringP("debug", "p", "products", "products output folder")
}
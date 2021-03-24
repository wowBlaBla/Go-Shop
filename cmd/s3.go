package cmd

import (
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"os"
)

var s3Cmd = &cobra.Command{
	Use:   "s3",
	Short: "AWS S3",
	Long:  `Publish resources to AWS S3`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Infof("AWS S3 module")
		accessKeyID := cmd.Flag("AccessKeyID").Value.String()
		if accessKeyID == "" {
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(s3Cmd)
	s3Cmd.Flags().StringP("AccessKeyID", "k", "", "AWS IAM AccessKeyID")
	s3Cmd.Flags().StringP("SecretAccessKey", "s", "", "AWS IAM SecretAccessKey")
	s3Cmd.Flags().StringP("Region", "r", "eu-central-1", "AWS S3 Region")
	s3Cmd.Flags().StringP("Bucket", "b", "", "AWS S3 Bucket")
}
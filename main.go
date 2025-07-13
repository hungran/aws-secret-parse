package main

import (
	"context"
	"html/template"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	secretTagValue    string
	inputTemplateName string
	outputName        string
	region            string
	cfgFile           string
)

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config.yaml")
		viper.AddConfigPath(".")
	}
	viper.AutomaticEnv() // read in environment variables that match
	if err := viper.ReadInConfig(); err == nil {
		logrus.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	// Set log level
	level, err := logrus.ParseLevel(viper.GetString("log-level"))
	if err != nil {
		logrus.Warnf("Invalid log level '%s', defaulting to info", viper.GetString("log-level"))
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
}

func validateConfig() {
	secretTagValue = viper.GetString("aws-secret-tag-value")
	if len(secretTagValue) == 0 {
		logrus.Fatal("aws-secret-tag-value not set (env: AWS_SECRET_TAG_VALUE or flag)")
	}
	inputTemplateName = viper.GetString("application-config-file")
	if len(inputTemplateName) == 0 {
		logrus.Fatal("application-config-file not set (env: APPLICATION_CONFIG_FILE or flag)")
	}
	outputName = viper.GetString("application-config-outfile")
	if len(outputName) == 0 {
		logrus.Fatal("application-config-outfile not set (env: APPLICATION_CONFIG_OUTFILE or flag)")
	}
	region = viper.GetString("aws-region")
	if len(region) == 0 {
		logrus.Fatal("aws-region not set (env: AWS_REGION or flag)")
	}
}

func listSecretsWithFilter(filter string, sess *session.Session) (map[string]string, error) {
	// Use a context with timeout to make the list secrets request
	duration := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), duration)
	defer cancel()
	// Create a new Secrets Manager client with the provided session
	svc := secretsmanager.New(sess)

	// Set up the input for the list secrets request with the specified filter
	input := &secretsmanager.ListSecretsInput{
		Filters: []*secretsmanager.Filter{
			{
				Key:    aws.String("tag-key"),
				Values: []*string{aws.String("env")},
			},
			{
				Key:    aws.String("tag-value"),
				Values: []*string{aws.String(filter)},
			},
		},
	}

	result, err := svc.ListSecretsWithContext(ctx, input)

	if err != nil {
		logrus.WithError(err).Error("Failed to list secrets from AWS")
		return nil, err
	}

	// Create a map of secrets with the secret name as the key and the secret ARN as the value
	secrets := make(map[string]string)

	for _, s := range result.SecretList {
		secrets[*s.Name] = *s.ARN
	}

	// Get the secret values for each secret in the map
	for secretTagValue, secretArn := range secrets {
		secretValue, err := getSecretValueWithContext(secretArn, svc, ctx)

		if err != nil {
			logrus.WithFields(logrus.Fields{"secretArn": secretArn, "secretTagValue": secretTagValue}).WithError(err).Error("Failed to get secret value")
			return nil, err
		}

		secrets[secretTagValue] = secretValue
	}

	// Return the map of secrets with the secret values
	return secrets, nil
}
func getSecretValueWithContext(secretArn string, svc *secretsmanager.SecretsManager, ctx context.Context) (string, error) {
	// Set up the input for the get secret value request
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretArn),
	}

	result, err := svc.GetSecretValueWithContext(ctx, input)

	if err != nil {
		logrus.WithField("secretArn", secretArn).WithError(err).Fatal("Error when getting secret value")
	}

	// Return the secret value as a string
	return *result.SecretString, nil
}

func createFile(templateFile string, secrets map[string]string) {
	// Parse the template file to create a new template
	tmpl, err := template.ParseFiles(templateFile)

	if err != nil {
		logrus.WithField("templateFile", templateFile).WithError(err).Fatal("ParseFiles Error")
	}

	// Create a new output file to write the template output to
	outputFile, err := os.Create(outputName)

	if err != nil {
		logrus.WithField("outputName", outputName).WithError(err).Fatal("Error creating output file")
	}

	defer outputFile.Close()

	// Execute the template with the secrets map to create the output
	err = tmpl.Execute(outputFile, secrets)

	if err != nil {
		logrus.WithField("outputName", outputName).WithError(err).Fatal("Error executing template")
	}

	logrus.Infof("Template output has been written to: %s", outputName)
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "aws-secret-parse",
		Short: "Parse AWS secrets and render a template",
		Run: func(cmd *cobra.Command, args []string) {
			initConfig()
			validateConfig()
			sess, err := session.NewSessionWithOptions(session.Options{
				SharedConfigState: session.SharedConfigEnable,
				Config:            aws.Config{Region: &region},
			})
			if err != nil {
				logrus.WithError(err).Fatal("Error creating AWS session")
			}
			secrets, err := listSecretsWithFilter(secretTagValue, sess)
			if err != nil {
				logrus.WithError(err).Fatal("Error listing secrets with filter")
			}
			createFile(inputTemplateName, secrets)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is empty)")
	rootCmd.Flags().String("aws-secret-tag-value", "", "AWS secret tag value (env: AWS_SECRET_TAG_VALUE)")
	rootCmd.Flags().String("application-config-file", "", "Input template file (env: APPLICATION_CONFIG_FILE)")
	rootCmd.Flags().String("application-config-outfile", "", "Output file (env: APPLICATION_CONFIG_OUTFILE)")
	rootCmd.Flags().String("aws-region", "", "AWS region (env: AWS_REGION)")
	rootCmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error, fatal) (env: LOG_LEVEL)")

	viper.BindPFlag("log-level", rootCmd.Flags().Lookup("log-level"))
	viper.BindPFlag("aws-secret-tag-value", rootCmd.Flags().Lookup("aws-secret-tag-value"))
	viper.BindPFlag("application-config-file", rootCmd.Flags().Lookup("application-config-file"))
	viper.BindPFlag("application-config-outfile", rootCmd.Flags().Lookup("application-config-outfile"))
	viper.BindPFlag("aws-region", rootCmd.Flags().Lookup("aws-region"))

	viper.BindEnv("log-level", "LOG_LEVEL")
	viper.BindEnv("aws-secret-tag-value", "AWS_SECRET_TAG_VALUE")
	viper.BindEnv("application-config-file", "APPLICATION_CONFIG_FILE")
	viper.BindEnv("application-config-outfile", "APPLICATION_CONFIG_OUTFILE")
	viper.BindEnv("aws-region", "AWS_REGION")

	if err := rootCmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("Command execution failed")
	}
}

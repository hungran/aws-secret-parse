package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	secretName        string
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
	secretName = viper.GetString("aws-secret-name")
	if len(secretName) == 0 {
		logrus.Fatal("aws-secret-name not set (env: AWS_SECRET_NAME or flag)")
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

func listSecretsWithFilter(name string, sess *session.Session) (map[string]string, error) {
	// Use a context with timeout to make the list secrets request
	duration := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), duration)
	defer cancel()
	// Create a new Secrets Manager client with the provided session
	svc := secretsmanager.New(sess)

	// Set up the input for the list secrets request with the specified name
	input := &secretsmanager.ListSecretsInput{
		Filters: []*secretsmanager.Filter{
			{
				Key:    aws.String("name"),
				Values: []*string{aws.String(name)},
			},
		},
	}

	result, err := svc.ListSecretsWithContext(ctx, input)
	if err != nil {
		logrus.WithError(err).Error("Failed to list secrets from AWS")
		return nil, err
	}

	// Build a map of secretName -> secretArn
	secretArns := make(map[string]string)
	for _, s := range result.SecretList {
		secretArns[*s.Name] = *s.ARN
	}
	logrus.WithField("secrets", secretArns).Info("Secrets found")

	// Build the template context
	templateContext := make(map[string]string)
	for secretName, secretArn := range secretArns {
		secretValue, err := getSecretValueWithContext(secretArn, svc, ctx)
		if err != nil {
			logrus.WithFields(logrus.Fields{"secretArn": secretArn, "secretName": secretName}).WithError(err).Error("Failed to get secret value")
			return nil, err
		}

		// Try to parse as JSON and merge keys
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(secretValue), &parsed); err == nil {
			for k, v := range parsed {
				templateContext[k] = fmt.Sprintf("%v", v)
			}
		} else {
			templateContext[secretName] = secretValue
		}
	}
	logrus.WithField("secrets value:", templateContext).Info("Secrets found")
	return templateContext, nil
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
			secrets, err := listSecretsWithFilter(secretName, sess)
			if err != nil {
				logrus.WithError(err).Fatal("Error listing secrets with filter")
			}
			createFile(inputTemplateName, secrets)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is empty)")
	rootCmd.Flags().String("aws-secret-name", "", "AWS secret tag value (env: AWS_SECRET_NAME)")
	rootCmd.Flags().String("application-config-file", "", "Input template file (env: APPLICATION_CONFIG_FILE)")
	rootCmd.Flags().String("application-config-outfile", "", "Output file (env: APPLICATION_CONFIG_OUTFILE)")
	rootCmd.Flags().String("aws-region", "", "AWS region (env: AWS_REGION)")
	rootCmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error, fatal) (env: LOG_LEVEL)")

	viper.BindPFlag("log-level", rootCmd.Flags().Lookup("log-level"))
	viper.BindPFlag("aws-secret-name", rootCmd.Flags().Lookup("aws-secret-name"))
	viper.BindPFlag("application-config-file", rootCmd.Flags().Lookup("application-config-file"))
	viper.BindPFlag("application-config-outfile", rootCmd.Flags().Lookup("application-config-outfile"))
	viper.BindPFlag("aws-region", rootCmd.Flags().Lookup("aws-region"))

	viper.BindEnv("log-level", "LOG_LEVEL")
	viper.BindEnv("aws-secret-name", "AWS_SECRET_NAME")
	viper.BindEnv("application-config-file", "APPLICATION_CONFIG_FILE")
	viper.BindEnv("application-config-outfile", "APPLICATION_CONFIG_OUTFILE")
	viper.BindEnv("aws-region", "AWS_REGION")

	if err := rootCmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("Command execution failed")
	}
}

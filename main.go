package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

var (
	secretTagValue    string
	inputTemplateName string
	outputName        string
	region            string
)

func validateVars() {
	secretTagValue = os.Getenv("AWS_SECRET_MANAGER_NAME")
	if len(secretTagValue) == 0 {
		log.Fatalln("AWS_SECRET_MANAGER_NAME env not set")
	}

	inputTemplateName = os.Getenv("APPLICATION_CONFIG_FILE")
	if len(inputTemplateName) == 0 {
		log.Fatalln("APPLICATION_CONFIG_FILE env not set")
	}

	outputName = os.Getenv("APPLICATION_CONFIG_OUTFILE")
	if len(outputName) == 0 {
		log.Fatalln("APPLICATION_CONFIG_OUTFILE env not set")
	}

	region = os.Getenv("AWS_REGION")
	if len(region) == 0 {
		log.Fatalln("AWS_REGION env not set")
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
		fmt.Printf("Error when get secret value: %v\n", err)
		os.Exit(1)
	}

	// Return the secret value as a string
	return *result.SecretString, nil
}

func createFile(templateFile string, secrets map[string]string) {
	// Parse the template file to create a new template
	tmpl, err := template.ParseFiles(templateFile)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Create a new output file to write the template output to
	outputFile, err := os.Create(outputName)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	defer outputFile.Close()

	// Execute the template with the secrets map to create the output
	err = tmpl.Execute(outputFile, secrets)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println("Template output has been written to:", outputName)
}

func main() {
	validateVars()
	// Create a new session using the default AWS configuration
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	// Call the listSecretsWithFilter function with an input filter
	secrets, err := listSecretsWithFilter(secretTagValue, sess)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	createFile(inputTemplateName, secrets)
}

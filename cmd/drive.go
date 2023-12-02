/*
Copyright © 2023 Quentin Lemaire

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	MimeTypeFolder   string = "application/vnd.google-apps.folder"
	MimeTypeShortcut string = "application/vnd.google-apps.shortcut"
)

type driveServiceContextKey struct{}

// driveCmd represents the drive command
var driveCmd = &cobra.Command{
	Use:   "drive",
	Short: "Manage shortcut folder on Google Drive.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var authOption option.ClientOption
		credentialsFilePath, _ := cmd.Flags().GetString("service-account-key-file")
		credentialsBase64, _ := cmd.Flags().GetString("service-account-key")

		switch {
		case credentialsFilePath != "":
			authOption = option.WithCredentialsFile(credentialsFilePath)
		case credentialsBase64 != "":
			decodedCredentials, err := base64.StdEncoding.DecodeString(credentialsBase64)
			if err != nil {
				return fmt.Errorf("unable to decode credentials: %w", err)
			}

			authOption = option.WithCredentialsJSON(decodedCredentials)
		}

		svc, err := getDriveService(cmd.Context(), authOption)
		if err != nil {
			return fmt.Errorf("unable to retrieve drive service: %w", err)
		}

		cmd.SetContext(context.WithValue(cmd.Context(), driveServiceContextKey{}, svc))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(driveCmd)

	// flags
	driveCmd.PersistentFlags().String("service-account-key-file", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"), "path to the service account key file")
	driveCmd.PersistentFlags().String("service-account-key", "", "service account key (json and base64 encoded)")

	// required flags
	driveCmd.MarkFlagsMutuallyExclusive("service-account-key-file", "service-account-key")
	driveCmd.MarkFlagsOneRequired("service-account-key-file", "service-account-key")

	// opts
	_ = driveCmd.MarkPersistentFlagFilename("service-account-key-file", "json")
}

// findShortcutByName finds a shortcut folder by name
func findShortcutByName(ctx context.Context, srv *drive.Service, folderName string, parentFolderID string) (*drive.File, error) {
	return findByName(ctx, srv, folderName, parentFolderID, MimeTypeShortcut)
}

// findByName finds a file by name
func findByName(ctx context.Context, srv *drive.Service, folderName string, parentFolderID string, mimeType string) (*drive.File, error) {
	query := fmt.Sprintf("name = '%s'", folderName)
	query += " and trashed=false"

	if parentFolderID != "" {
		query += fmt.Sprintf(" and '%s' in parents", parentFolderID)
	}

	if mimeType != "" {
		query += fmt.Sprintf(" and mimeType='%s'", mimeType)
	}

	// Search for the folder
	r, err := srv.Files.
		List().
		Fields("files(name,id,mimeType,parents)").
		Context(ctx).
		Q(query).
		Corpora("allDrives").
		IncludeTeamDriveItems(true).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to search for file: %w", err)
	}

	if len(r.Files) == 0 {
		return nil, fmt.Errorf("no file found with name '%s'", folderName)
	}

	return r.Files[0], nil
}

// getDriveService returns a Google Drive service
func getDriveService(ctx context.Context, authOption ...option.ClientOption) (*drive.Service, error) {
	return drive.NewService(ctx, authOption...)
}

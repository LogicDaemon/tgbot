package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Secrets holds only the authentication secrets for the bot
type Secrets struct {
	TelegramBotToken string `json:"telegram_bot_token"`
}

// Settings holds the configuration settings for the bot
type Settings struct {
	TelegramChannelID int64 `json:"telegram_channel_id"`
}

// Article represents a news item
type Article struct {
	Title          string
	URL            string
	Date           string
	Text           string
	SectionContent string
}

const (
	magticomNewsURL = "https://www.magticom.ge/en/about-company/news"
	dbFileName      = "posted_articles.sqlite3"
)

func getLocalAppDataDir() string {
	// Default paths based on OS
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			log.Panicf("LOCALAPPDATA environment variable is not set")
		}
		return localAppData
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Panicf("Error getting home directory: %v", err)
		}
		return filepath.Join(homeDir, ".local")
	}
}

func getDefaultSecretsPath() string {
	var secretDataDir string

	// Check environment variable
	if envPath := os.Getenv("SECRETS_PATH"); envPath != "" {
		return envPath
	}

	// Check SecretDataDir environment variable
	if dir := os.Getenv("SecretDataDir"); dir != "" {
		secretDataDir = dir
	} else {
		secretDataDir = filepath.Join(getLocalAppDataDir(), "_sec")
	}

	return filepath.Join(secretDataDir, "repost_magti_news.json")
}

func getDBPath() string {
	dataDir := filepath.Join(getLocalAppDataDir(), "repost_magti_news")

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	return filepath.Join(dataDir, dbFileName)
}

func initDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	// Create table with URL and timestamp
	query := `
    CREATE TABLE IF NOT EXISTS posted_articles (
        url TEXT PRIMARY KEY,
        posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    `
	_, err = db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	return db, nil
}

func isArticlePosted(db *sql.DB, url string) (bool, error) {
	query := "SELECT 1 FROM posted_articles WHERE url = ?"
	var exists int
	err := db.QueryRow(query, url).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("error checking if article posted: %w", err)
	}
	return true, nil
}

func markArticleAsPosted(db *sql.DB, url string) error {
	query := "INSERT INTO posted_articles (url) VALUES (?)"
	_, err := db.Exec(query, url)
	return err
}

func removeOldArticles(db *sql.DB) error {
	// Delete articles older than one year
	query := `DELETE FROM posted_articles WHERE posted_at < datetime('now', '-1 year')`

	result, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("error removing old articles: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting affected rows: %w", err)
	}

	if rowsAffected > 0 {
		log.Printf("Removed %d old articles from database", rowsAffected)
	}

	return nil
}

func getSettingsPath() string {
	var dataDir string

	// Default paths based on OS
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		dataDir = filepath.Join(localAppData, "repost_magti_news")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting home directory: %v", err)
		}
		dataDir = filepath.Join(homeDir, ".local", "repost_magti_news")
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	return filepath.Join(dataDir, "settings.json")
}

func loadFile(filePath string, displayType string) []byte {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Panicf(`%s file not found at "%s"`, displayType, filePath)
	}

	rawdata, err := os.ReadFile(filePath)
	if err != nil {
		log.Panicf(`error %v reading %s file "%s"`, err, displayType, filePath)
	}

	return rawdata
}

func loadSecrets() (*Secrets, error) {
	var secrets Secrets

	if err := json.Unmarshal(loadFile(getDefaultSecretsPath(), "secrets"), &secrets); err != nil {
		return nil, fmt.Errorf("error parsing secrets file: %v", err)
	}

	if secrets.TelegramBotToken == "" {
		return nil, fmt.Errorf("missing required secrets")
	}

	return &secrets, nil
}

func loadSettings() (*Settings, error) {
	var settings Settings

	if err := json.Unmarshal(loadFile(getSettingsPath(), "settings"), &settings); err != nil {
		return nil, fmt.Errorf("error parsing settings file: %v", err)
	}

	if settings.TelegramChannelID == 0 {
		return nil, fmt.Errorf("missing required settings")
	}

	return &settings, nil
}

func sendToTelegram(botToken string, channelID int64, article Article) error {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return fmt.Errorf("error initializing bot: %v", err)
	}

	// Format the date nicely
	dateText := strings.TrimSpace(article.Text)

	// The section content is already formatted by parseHtmlContent
	content := article.SectionContent

	// Format message with proper spacing
	message := fmt.Sprintf("📅 %s\n\n%s\n\n🔗 %s",
		dateText, content, article.URL)

	// Use plain text mode
	msg := tgbotapi.NewMessageToChannel(fmt.Sprintf("%d", channelID), message)

	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("error sending message: %v", err)
	}

	return nil
}

func fetchWonderDaysNews() ([]Article, error) {
	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	// Get the news listing page
	resp, err := client.Get(magticomNewsURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching news page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	var wonderDaysItems []Article

	// Find news items in the specified selector
	doc.Find(".post-listing a.post-list-item").Each(func(i int, s *goquery.Selection) {
		url, exists := s.Attr("href")
		if exists {
			// Make relative URLs absolute
			if !strings.HasPrefix(url, "http") {
				url = "https://www.magticom.ge/" + strings.TrimPrefix(url, "/")
			}

			// Extract date from the post listing
			dateStr := s.Find(".post-date").Text()
			titleStr := s.Find(".post-content").Text()

			article := Article{
				Title: strings.TrimSpace(titleStr),
				URL:   url,
				Date:  dateStr,
			}

			// Fetch the full article content
			content, sectionContent, err := fetchArticleContent(client, url)
			if err != nil {
				log.Printf("Warning: couldn't fetch article content: %v", err)
			} else {
				article.Text = content
				article.SectionContent = sectionContent
			}

			wonderDaysItems = append(wonderDaysItems, article)
		}
	})

	return wonderDaysItems, nil
}

// parseHtmlContent extracts text from HTML content with proper formatting
func parseHtmlContent(htmlContent string) string {
	reader := strings.NewReader(htmlContent)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		// If parsing fails, return the original content
		return htmlContent
	}

	var result strings.Builder

	// Process paragraphs and lists
	doc.Find("p, ul, li").Each(func(i int, s *goquery.Selection) {
		// Get the tag name
		tagName := goquery.NodeName(s)

		// Clean up the text
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		// Replace Georgian Lari icon with the proper symbol
		if s.Find("span.icon-gel").Length() > 0 {
			text = strings.TrimSpace(text) + " ₾"
		}

		switch tagName {
		case "p":
			// For paragraphs, add text followed by two newlines
			result.WriteString(text)
			result.WriteString("\n\n")
		case "ul":
			// Don't process ul directly, we'll handle li elements
		case "li":
			// For list items, add a bullet point
			result.WriteString("• ")
			result.WriteString(text)
			result.WriteString("\n")
		}
	})

	// Clean up the result
	content := result.String()

	// Replace HTML entities
	content = strings.ReplaceAll(content, "&nbsp;", " ")

	// Clean up any excess newlines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

func fetchArticleContent(client *http.Client, url string) (string, string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", err
	}

	// Extract the date content
	dateContent := doc.Find("#article > article > div > div").Text()

	// Extract the section content as HTML
	sectionHtml, err := doc.Find("#article > article > div > section").Html()
	if err != nil {
		// If we can't get the HTML, fall back to text
		sectionContent := doc.Find("#article > article > div > section").Text()
		return strings.TrimSpace(dateContent), strings.TrimSpace(sectionContent), nil
	}

	// Parse the HTML content
	sectionContent := parseHtmlContent(sectionHtml)

	return strings.TrimSpace(dateContent), sectionContent, nil
}

func printInstructions() {
	fmt.Println("Missing required configuration for the Magticom News Reposter.")
	fmt.Println("\nPlease create the following configuration files:")

	// Secrets file
	fmt.Println("\n1. Secrets file (for the bot token):")
	fmt.Printf("   Path: %s\n", getDefaultSecretsPath())
	fmt.Println("   Format:")
	fmt.Println(`   {
     "telegram_bot_token": "YOUR_TELEGRAM_BOT_TOKEN"
     }`)
	fmt.Println("   To obtain, create a Telegram bot by talking to @BotFather and get the token")

	// Settings file
	fmt.Println("\n2. Settings file (for the channel ID):")
	fmt.Printf("   Path: %s\n", getSettingsPath())
	fmt.Println("   Format:")
	fmt.Println(`   {
     "telegram_channel_id": YOUR_CHANNEL_ID_NUMBER
   }`)
	fmt.Println("   To get it, add your bot to the target channel as an administrator,")
	fmt.Println("   and forward a message from the channel to @userinfobot.")
	fmt.Println("   Use the 'Id' number from the 'Forwarded from chat' value (including the negative sign)")
}

// Run executes the service
func Run() {
	secrets, err := loadSecrets()
	if err != nil {
		log.Printf("Error loading secrets: %v", err)
		printInstructions()
		return
	}

	settings, err := loadSettings()
	if err != nil {
		log.Printf("Error loading settings: %v", err)
		printInstructions()
		return
	}

	dbPath := getDBPath()
	db, err := initDB(dbPath)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	if err := removeOldArticles(db); err != nil {
		log.Printf("Warning: Error removing old articles: %v", err)
	}

	log.Println("Fetching news from Magticom...")
	articles, err := fetchWonderDaysNews()
	if err != nil {
		log.Fatalf("Error fetching news: %v", err)
	}

	if len(articles) == 0 {
		log.Println("No news found on the page.")
		return
	}

	var newItemsToPost []Article
	// News items are fetched newest first. We iterate to find new ones.
	for _, item := range articles {
		posted, err := isArticlePosted(db, item.URL)
		if err != nil {
			log.Printf("Error checking if article was posted (%s): %v. Skipping.", item.URL, err)
			continue
		}
		if !posted {
			newItemsToPost = append(newItemsToPost, item)
		}
	}

	if len(newItemsToPost) > 0 {
		log.Printf("Found %d new items to post.", len(newItemsToPost))

		// Post items from oldest to newest (reverse the slice of new items)
		for i := len(newItemsToPost) - 1; i >= 0; i-- {
			item := newItemsToPost[i]
			log.Printf("Posting to Telegram: %s (%s)", item.Title, item.URL)

			if err := sendToTelegram(secrets.TelegramBotToken, settings.TelegramChannelID, item); err != nil {
				log.Printf("Error sending to Telegram (%s): %v", item.URL, err)
				// If sending fails, we don't mark it as posted, so it will be retried next time.
				continue
			}

			log.Printf("Successfully posted: %s", item.URL)
			if err := markArticleAsPosted(db, item.URL); err != nil {
				log.Printf(
					"Error marking article %s as posted: %v.",
					item.URL, err)
			}
		}
	} else {
		log.Println("No new news to post.")
	}
}

func main() {
	Run()
}

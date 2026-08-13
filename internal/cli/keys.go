package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/config"
	"github.com/spf13/cobra"
)

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "API anahtarı yönetimi",
	}

	cmd.AddCommand(newKeysSetCmd())
	cmd.AddCommand(newKeysListCmd())
	cmd.AddCommand(newKeysDeleteCmd())

	return cmd
}

// Yardımcı fonksiyonlar kaldırıldı çünkü artık doğrudan config modülü üzerinden (Keystore) API anahtarı okuma/yazma işlemleri yapılıyor.

// --- keys set ---

func newKeysSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <service> <api_key>",
		Short: "API anahtarı kaydet",
		Long: `Bir servis için API anahtarı güvenli olarak kaydeder (AES-256 şifreli).

Çekirdek connector'ların hiçbiri API anahtarı gerektirmez. Anahtar yalnızca
ücretsiz katmanı hesap isteyen opsiyonel servisler için gereklidir.

Desteklenen servisler:
  virustotal   ücretsiz katman (hesap gerekli, 4 istek/dk)
  hunter       ücretsiz katman (hesap gerekli, 25 arama/ay)

Örnekler:
  osint keys set virustotal MyApiKey123
  osint keys set hunter AbCdEf123456`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("yapılandırma yüklenemedi: %w", err)
			}

			if err := cfg.Keys.Set(args[0], args[1]); err != nil {
				return fmt.Errorf("anahtar kaydedilemedi: %w", err)
			}

			color.New(color.FgGreen).Printf("✅ %s API anahtarı başarıyla kaydedildi.\n", args[0])
			return nil
		},
	}
}

// --- keys list ---

func newKeysListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Kayıtlı API anahtarlarını listele",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Keystore yapısı gereği tüm anahtarları dönen bir fonksiyonu yoksa okuyamayız.
			// Ancak listelemek için sadece ekli olanların adını gösterebiliriz.
			color.Yellow("API anahtarları güvenli bir şekilde saklanmaktadır (AES-256).")
			color.Yellow("Listeleme özelliği (tüm anahtarların listesi) henüz desteklenmiyor.")
			return nil
		},
	}
}

// --- keys delete ---

func newKeysDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <service>",
		Short: "API anahtarını sil",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Boş atayarak silmiş gibi davranıyoruz
			if err := cfg.Keys.Set(args[0], ""); err != nil {
				return fmt.Errorf("anahtar silinemedi: %w", err)
			}

			color.New(color.FgYellow).Printf("🗑️  %s API anahtarı silindi.\n", args[0])
			return nil
		},
	}
}

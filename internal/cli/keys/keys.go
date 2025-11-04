package keys

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	myjwt "smartorders/user-svc/internal/auth/jwt"
)

// Run — точка входа CLI-команды "keys".
//
// Поддерживаемые флаги:
//
//	-ls         — выводит список всех существующих наборов ключей с их статусом.
//
//	-new NAME   — создаёт новый набор ключей с указанным именем (в неактивном состоянии).
//
//	-use ID     — активирует указанный набор ключей эксклюзивно (деактивирует остальные).
//
//	-add ID     — активирует указанный набор ключей неэксклюзивно (оставляя активными другие).
//
//	-jwks       — выводит JSON Web Key Set (JWKS) только для активных ключей.
//
//	-store PATH — переопределяет путь к хранилищу ключей (переменная SMARTORDERS_KEYSTORE_PATH).
//
//	-h          — отображает справку по доступным флагам.
//
// Переменные окружения:
//
//	SMARTORDERS_KEYSTORE_MASTER_KEY   — мастер-ключ (32 байта или base64), обязателен.
//	SMARTORDERS_KEYSTORE_PATH         — путь к зашифрованному файлу-хранилищу ключей.
//	SMARTORDERS_KEYS_DIR              — каталог для хранения служебных данных ключей.
func Run(args []string) error {
	fs := flag.NewFlagSet("keys", flag.ExitOnError)

	flagList := fs.Bool("ls", false, "List key sets")
	flagCreate := fs.String("new", "", "Create key set with given name (inactive)")
	flagUse := fs.String("use", "", "Activate key set exclusively (deactivate others)")
	flagAdd := fs.String("add", "", "Activate key set non-exclusively (keep others active)")
	flagJWKS := fs.Bool("jwks", false, "Print JWKS (active only)")
	flagStore := fs.String("store", "", "Override keystore path (SMARTORDERS_KEYSTORE_PATH)")
	flagHelp := fs.Bool("h", false, "Show help")

	_ = fs.Parse(args)

	if *flagHelp || (!*flagList && *flagCreate == "" && *flagUse == "" && *flagAdd == "" && !*flagJWKS) {
		usage()
		return nil
	}

	if *flagStore != "" {
		_ = os.Setenv("SMARTORDERS_KEYSTORE_PATH", *flagStore)
	}

	switch {
	case *flagList:
		sets, err := myjwt.ListKeySets()
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(sets, "", "  ")
		fmt.Println(string(b))
		return nil

	case *flagCreate != "":
		name, kid, err := myjwt.CreateKeySet(*flagCreate)
		if err != nil {
			return err
		}
		fmt.Printf("created key set: name=%s kid=%s (inactive)\n", name, kid)
		return nil

	case *flagUse != "":
		if err := myjwt.ActivateKeySet(*flagUse, true); err != nil {
			return err
		}
		fmt.Println("activated (exclusive)")
		return nil

	case *flagAdd != "":
		if err := myjwt.ActivateKeySet(*flagAdd, false); err != nil {
			return err
		}
		fmt.Println("activated (non-exclusive)")
		return nil

	case *flagJWKS:
		b, err := myjwt.ExportJWKS(true)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	return nil
}

// usage — выводит краткую справку по флагам и переменным окружения,
// если команда запущена без аргументов или с флагом -h.
func usage() {
	fmt.Println(`user-svc keys - key management

ENV:
  SMARTORDERS_KEYSTORE_MASTER_KEY   # 32-byte key or base64; required
  SMARTORDERS_KEYSTORE_PATH         # optional file path override
  SMARTORDERS_KEYS_DIR              # optional directory override

COMMANDS:
  user-svc keys -ls
  user-svc keys -new "NAME"
  user-svc keys -use "NAME|KID"        # exclusive activation (deactivate others)
  user-svc keys -add "NAME|KID"        # non-exclusive activation (keep others active)
  user-svc keys -jwks
  user-svc keys -store "/var/lib/smartorders/keystore.enc"

EXAMPLE:
  SMARTORDERS_KEYSTORE_MASTER_KEY="0123456789_0123456789_0123456789__" \
  user-svc keys -new "prod-` + time.Now().Format("2006-01") + `"
`)
}

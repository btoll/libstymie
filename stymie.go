// Copyright © 2017 Benjamin Toll <ben@benjamintoll.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package libstymie

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/btoll/diceware"
	"github.com/btoll/sillypass"
)

type Encrypter interface {
	Configure() error
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

func configure(e Encrypter) error {
	return e.Configure()
}

func decrypt(e Encrypter, b []byte) ([]byte, error) {
	return e.Decrypt(b)
}

func encrypt(e Encrypter, b []byte) ([]byte, error) {
	return e.Encrypt(b)
}

type Key struct {
	Fields map[string]string `json:"fields"`
}

type PassConfig struct {
	Diceware  int `json:"diceware"`  // Number of words in a Diceware passphrase.
	Sillypass int `json:"sillypass"` // Number of characters in a Sillypass password.
}

func formatError(err error) error {
	return fmt.Errorf("[ERROR] %v\n", err)
}

func getKeyFile() string {
	return getStymieDir() + "/k"
}

func getStymieDir() string {
	var path string

	if path = os.Getenv("STYMIE"); path == "" {
		path = os.Getenv("HOME")
	}

	return path + "/.stymie.d"
}

type Stymie struct {
	Dir    string    `json:"dir"`
	Plugin Encrypter `json:"plugin"`
	// TODO: Why can't PassConfig be a pointer? 20180603
	PassConfig PassConfig      `json:"passConfig"`
	Keys       map[string]*Key `json:"keys"`
}

func New(e Encrypter) *Stymie {
	return &Stymie{
		Dir:    getStymieDir(),
		Plugin: e,
		Keys:   map[string]*Key{},
		PassConfig: PassConfig{
			Diceware:  6,
			Sillypass: 12,
		},
	}
}

func (s *Stymie) addNewFields(k *Key) *Key {
	var str, n, v string

	fmt.Print("Create another field? [y/N]: ")
	fmt.Scanf("%s", &str)
	switch str {
	case "y":
		fallthrough
	case "Y":
		for {
			fmt.Print("Name: ")

			if _, err := fmt.Scanf("%s", &n); err != nil {
				fmt.Println("Cannot be blank.")
			} else {
				fmt.Print("Value: ")

				if _, err := fmt.Scanf("%s", &v); err != nil {
					fmt.Println("Cannot be blank.")
				} else {
					k.Fields[n] = v
					break
				}
			}
		}

		s.addNewFields(k)
	}

	return k
}

func (s *Stymie) Configure() error {
	var str string
	var i int

	fmt.Print("Enter the full path of the directory into which `stymie` will install .stymie.d [~]: ")
	fmt.Scanf("%s", &str)
	if str != "" {
		s.Dir = str + "/.stymie.d"
	}

	// Pass off to a user-configured method to determine its own customization.
	err := configure(s.Plugin)
	if err != nil {
		return err
	}

	fmt.Print("How many words should Diceware use to generate a passphrase? [6]: ")
	fmt.Scanf("%s", &str)
	i, _ = strconv.Atoi(str)
	// Equals zero if there was an error, such as a user entering characters that
	// couldn't be converted to an integer.
	// So, don't check for and return an error.
	if i != 0 {
		s.PassConfig.Diceware = i
	}

	fmt.Print("How many characters should Sillypass use to generate a password? [12]: ")
	fmt.Scanf("%s", &str)
	// Equals zero if there was an error, such as a user entering characters that
	// couldn't be converted to an integer.
	// So, don't check for and return an error.
	i, _ = strconv.Atoi(str)
	if i != 0 {
		s.PassConfig.Sillypass = i
	}

	return nil
}

func (s *Stymie) Decrypt(b []byte) ([]byte, error) {
	return decrypt(s.Plugin, b)
}

func (s *Stymie) Encrypt(b []byte) ([]byte, error) {
	return encrypt(s.Plugin, b)
}

func (s *Stymie) GeneratePassphrase(fn func() string) string {
	var t string
	str := fn()
	fmt.Println(str)

	fmt.Print("Accept? [Y/n]: ")
	fmt.Scanf("%s", &t)
	switch t {
	case "n":
		fallthrough
	case "N":
		return s.GeneratePassphrase(fn)
	default:
		// Remove spaces (nop for Sillypass).
		return strings.Replace(str, " ", "", -1)
	}

	return ""
}

func (s *Stymie) GetFileContents() error {
	// Maybe pass filename is as func param?
	keyfile := getKeyFile()

	b, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return formatError(err)
	}

	decrypted, err := s.Decrypt(b)
	if err != nil {
		return err
	}

	// Fill the `stymie` struct with the decrypted json.
	err = json.Unmarshal(decrypted, s)
	if err != nil {
		return formatError(err)
	}

	return nil
}

func (s *Stymie) GetKeyFields() *Key {
	k := &Key{
		Fields: make(map[string]string),
	}

	for {
		var str string

		fmt.Print("URL: ")
		// Note that we don't care if there's an error here!
		fmt.Scanf("%s", &str)

		k.Fields["url"] = str

		for {
			fmt.Print("Username: ")
			if _, err := fmt.Scanf("%s", &str); err != nil {
				fmt.Println("Cannot be blank.")
			} else {
				k.Fields["username"] = str
				break
			}
		}

		fmt.Println("Password generation method:")
		k.Fields["password"] = s.GetPassword()
		k = s.addNewFields(k)
		break
	}

	return k
}

func (s *Stymie) GetPassword() string {
	var str string

	fmt.Println("\t(1) Diceware (passphrase)")
	fmt.Println("\t(2) Sillypass (mixed-case, alphanumeric, random characters)")
	fmt.Println("\t(3) I'll generate it myself")
	fmt.Println("\t(4) Skip")
	fmt.Print("Select [1]: ")
	fmt.Scanf("%s", &str)
	switch str {
	case "2":
		// TODO
		return s.GeneratePassphrase(func() string {
			return sillypass.Generate(s.PassConfig.Sillypass)
		})
	case "3":
		for {
			fmt.Print("Custom password: ")
			if _, err := fmt.Scanf("%s", &str); err != nil {
				fmt.Println("Cannot be blank.")
			} else {
				return str
				break
			}
		}
	case "4":
		break
	default:
		return s.GeneratePassphrase(func() string {
			return diceware.Generate(s.PassConfig.Diceware, "")
		})
		//			k.generatePassphrase(diceware.Generate)
	}

	return ""
}

func (s *Stymie) GetUpdatedFields(k *Key) *Key {
	newkey := &Key{
		Fields: make(map[string]string),
	}

	for key, value := range k.Fields {
		var newvalue string

		if key == "password" {
			var str string
			fmt.Printf("Edit %s (%s):\n", key, value)
			if str = s.GetPassword(); str == "" {
				str = value
			}
			newkey.Fields[key] = str
		} else {
			fmt.Printf("Edit %s (%s): ", key, value)
			// Usually, an error here means that nothing was entered (just a newline, e.g. [Enter]).
			if _, err := fmt.Scanf("%s", &newvalue); err != nil {
				newkey.Fields[key] = value
			} else {
				newkey.Fields[key] = newvalue
			}
		}
	}

	return s.addNewFields(newkey)
}

func (s *Stymie) Init() error {
	err := s.Configure()
	if err != nil {
		return err
	}
	err = s.MakeDir()
	if err != nil {
		return err
	}
	err = s.MakeConfigFile()
	if err != nil {
		return err
	}
	return nil
}

func (s *Stymie) MakeConfigFile() error {
	f, err := os.Create(s.Dir + "/k")
	defer f.Close()
	if err != nil {
		return formatError(err)
	}

	stymieConfig, err := json.Marshal(s)
	encrypted, err := s.Encrypt([]byte(stymieConfig))
	if err != nil {
		return err
	}
	f.Write(encrypted)

	if err != nil {
		return formatError(err)
	}

	return nil
}

func (s *Stymie) MakeDir() error {
	_, err := os.Stat(s.Dir)
	if !os.IsNotExist(err) {
		return errors.New(fmt.Sprintf("%s already exists, exiting.", s.Dir))
	}
	return os.Mkdir(s.Dir, 0700)
}

func (s *Stymie) PutFileContents() error {
	// Back to json (maybe combine this with the actual encryption?).
	b, err := json.Marshal(s)
	if err != nil {
		return formatError(err)
	}

	//	// Pretty-print the json.
	//	var out bytes.Buffer
	//	json.Indent(&out, b, "", "\t")
	//
	//	encrypted, err := s.Encrypt(out.Bytes())
	encrypted, err := s.Encrypt(b)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(getKeyFile(), encrypted, 0700)
	if err != nil {
		return formatError(err)
	}

	return nil
}

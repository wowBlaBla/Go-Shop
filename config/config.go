package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	DEFAULT_HOST = ""
	DEFAULT_PORT = 18092
	DEFAULT_HTTPS_PORT = 18492
	DEFAULT_HUGO = "hugo"
	DEFAULT_DATABASE_URI = "database/sqlite/database.sqlite"
)

var (
	validFrom  = ""
	validFor   = 10 * 365 * 24 * time.Hour
	isCA       = true
	rsaBits    = 2048
	ecdsaCurve = ""
)

func NewConfig(file string) *Config {
	config := &Config{
		path: file,
	}
	return config
}

type Config struct {
	path string
	//
	Base  string
	Host  string
	Port  int
	Debug bool
	Https struct {
		Enabled bool
		Host string
		Port int
		Crt string
		Key string
	}
	Preview string
	//
	Database struct {
		Dialer string // "mysql" or "sqlite"
		Uri string // "root:password@/db_name?charset=utf8&parseTime=True&loc=Local" or ""
	}
	I18n struct {
		Enabled bool
		Languages []Language
	}
	//
	Products string
	//
	Resize ResizeConfig
	//
	Hugo struct {
		Bin    string
		Theme  string
		Minify bool
	}
	Wrangler WranglerConfig
	//
	Currency string // usd, eur
	//
	Payment PaymentConfig
	Notification NotificationConfig
	Swagger struct {
		Enabled bool
		Url string
	}
	Modified time.Time
}

type WranglerConfig struct {
	Enabled bool
	Bin string
	ApiToken string
}

type PaymentConfig struct {
	Enabled bool
	Default string
	Stripe struct {
		Enabled bool
		PublishedKey string
		SecretKey string
	}
	Mollie struct {
		Enabled bool
		Key string
		ProfileID string
		Methods string
	}
	AdvancePayment struct {
		Enabled bool
		Details string `json:",omitempty"`
	}
	OnDelivery struct {
		Enabled bool
	}
	VAT float64
}

type ResizeConfig struct {
	Enabled bool
	Thumbnail struct {
		Enabled bool
		Size string //64x0,128x0
	}
	Image struct {
		Enabled bool
		Size string // 128x0,256x0
	}
	Quality int
}

type NotificationConfig struct {
	Enabled bool
	Email EmailConfig
}

type EmailConfig struct {
	Enabled bool
	Name string
	Email string
	Key string // SendGrid key
}

func (c *Config) Save() error {
	c.Modified = time.Now()
	if strings.LastIndex(c.path, ".json") == len(c.path) - 5 {
		if bts, err := json.MarshalIndent(c, "", "   "); err == nil {
			return ioutil.WriteFile(c.path, bts, 0755)
		}else{
			return err
		}
	} else if strings.LastIndex(c.path, ".toml") == len(c.path) - 5 {
		if file, err := os.Create(c.path); err == nil {
			defer file.Close()
			e := toml.NewEncoder(file)
			return e.Encode(c)
		}else{
			return err
		}
	}
	return errors.New("unknown file type")
}

type Language struct {
	Enabled bool
	Name string
	Code string
	Suffix string `toml:",omitempty"`
}

func GenerateSSL(crtPath string, keyPath string, host string) error {
	var priv interface{}
	var err error
	switch ecdsaCurve {
	case "":
		priv, err = rsa.GenerateKey(rand.Reader, rsaBits)
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		err = fmt.Errorf("unrecognized elliptic curve: %v", ecdsaCurve)
		return err
	}
	if err != nil {
		return err
	}

	var notBefore time.Time
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			return err
		}
	}

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	name := strings.Split(host, ":")[0]
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: name,
			Organization: []string{name},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(crtPath)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}
	certOut.Close()

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		err = fmt.Errorf("failed to open %v for writing: %v", keyPath, err)
		return err
	}
	pemBlock, err := pemBlockForKey(priv)
	if err != nil {
		err = fmt.Errorf("unable to marshal ECDSA private key: %v", err)
		return err
	}
	if err := pem.Encode(keyOut, pemBlock); err != nil {
		return err
	}
	keyOut.Close()
	return nil
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) (*pem.Block, error) {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}, nil
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}, nil
	default:
		return nil, nil
	}
}
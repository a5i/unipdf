/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package sighandler

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/unidoc/pkcs7"
	"github.com/unidoc/timestamp"
	"github.com/unidoc/unipdf/v3/core"
	"github.com/unidoc/unipdf/v3/model"
	"golang.org/x/crypto/ocsp"
)

// PAdESLevelB contains parameters for PAdES B-Level signature creation.
// ETSI TS 103 172 V2.2.2 (2013-04) page 10.
type PAdESLevelB struct {
	PrivateKey  *rsa.PrivateKey
	Certificate *x509.Certificate
	CaCert      *x509.Certificate
}

// New creates a new Adobe.PPKLite ETSI.CAdES.detached Level B signature handler.
// All fields are required.
func (p *PAdESLevelB) New() (padesSignatureHandler, error) {
	if p.PrivateKey == nil {
		return nil, fmt.Errorf("field PrivateKey is required")
	}
	if p.Certificate == nil {
		return nil, fmt.Errorf("field Certificate is required")
	}
	if p.CaCert == nil {
		return nil, fmt.Errorf("field CaCert is required")
	}
	return &etsiPAdES{
		certificate: p.Certificate,
		privateKey:  p.PrivateKey,
		caCert:      p.CaCert,
	}, nil
}

// PAdESLevelT contains parameters for PAdES T-Level signature creation.
// ETSI TS 103 172 V2.2.2 (2013-04) page 11.
type PAdESLevelT struct {
	PrivateKey                    *rsa.PrivateKey
	Certificate                   *x509.Certificate
	CaCert                        *x509.Certificate
	CertificateTimestampServerURL string
}

// New creates a new Adobe.PPKLite ETSI.CAdES.detached Level T signature handler.
// All fields are required.
func (p *PAdESLevelT) New() (padesSignatureHandler, error) {
	if p.PrivateKey == nil {
		return nil, fmt.Errorf("field PrivateKey is required")
	}
	if p.Certificate == nil {
		return nil, fmt.Errorf("field Certificate is required")
	}
	if p.CaCert == nil {
		return nil, fmt.Errorf("field CaCert is required")
	}
	if p.CertificateTimestampServerURL == "" {
		return nil, fmt.Errorf("field CertificateTimestampServerURL is required")
	}
	return &etsiPAdES{
		certificate:        p.Certificate,
		privateKey:         p.PrivateKey,
		caCert:             p.CaCert,
		timestampServerURL: p.CertificateTimestampServerURL,
	}, nil
}

// PAdESLevelLT contains parameters for PAdES LTV/LT-Level signature creation.
// ETSI TS 103 172 V2.2.2 (2013-04) page 12.
type PAdESLevelLT struct {
	PrivateKey                    *rsa.PrivateKey
	Certificate                   *x509.Certificate
	CaCert                        *x509.Certificate
	CertificateTimestampServerURL string
	CLRDistributionPoints         []string
	OCSPServers                   []string
}

// New creates a new Adobe.PPKLite ETSI.CAdES.detached Level LT (PAdES LTV) signature handler.
// All fields are required.
func (p *PAdESLevelLT) New() (padesSignatureHandler, error) {
	if p.PrivateKey == nil {
		return nil, fmt.Errorf("field PrivateKey is required")
	}
	if p.Certificate == nil {
		return nil, fmt.Errorf("field Certificate is required")
	}
	if p.CaCert == nil {
		return nil, fmt.Errorf("field CaCert is required")
	}
	if p.CertificateTimestampServerURL == "" {
		return nil, fmt.Errorf("field CertificateTimestampServerURL is required")
	}
	if len(p.CLRDistributionPoints) == 0 {
		return nil, fmt.Errorf("field CLRDistributionPoints is required")
	}
	if len(p.OCSPServers) == 0 {
		return nil, fmt.Errorf("field OCSPServers is required")
	}
	return &etsiPAdES{
		privateKey:            p.PrivateKey,
		certificate:           p.Certificate,
		caCert:                p.CaCert,
		crlDistributionPoints: p.CLRDistributionPoints,
		ocspServers:           p.OCSPServers,
		timestampServerURL:    p.CertificateTimestampServerURL,
	}, nil
}

// PAdESLevelLTA contains parameters for PAdES LTA-Level signature creation.
// ETSI TS 103 172 V2.2.2 (2013-04) page 13.
type PAdESLevelLTA struct {
	PrivateKey                    *rsa.PrivateKey
	Certificate                   *x509.Certificate
	CaCert                        *x509.Certificate
	CertificateTimestampServerURL string
	CLRDistributionPoints         []string
	OCSPServers                   []string
	TimestampServerURL            string
}

// New creates a new Adobe.PPKLite ETSI.CAdES.detached Level LT (PAdES LTV) signature handler.
// All fields are required.
func (p *PAdESLevelLTA) New() (padesSignatureHandler, model.SignatureHandler, error) {
	if p.PrivateKey == nil {
		return nil, nil, fmt.Errorf("field PrivateKey is required")
	}
	if p.Certificate == nil {
		return nil, nil, fmt.Errorf("field Certificate is required")
	}
	if p.CaCert == nil {
		return nil, nil, fmt.Errorf("field CaCert is required")
	}
	if p.CertificateTimestampServerURL == "" {
		return nil, nil, fmt.Errorf("field CertificateTimestampServerURL is required")
	}
	if len(p.CLRDistributionPoints) == 0 {
		return nil, nil, fmt.Errorf("field CLRDistributionPoints is required")
	}
	if len(p.OCSPServers) == 0 {
		return nil, nil, fmt.Errorf("field OCSPServers is required")
	}
	if p.TimestampServerURL == "" {
		return nil, nil, fmt.Errorf("field TimestampServerURL is required")
	}
	handler, err := NewDocTimeStamp(p.TimestampServerURL, crypto.SHA512)
	if err != nil {
		return nil, nil, err
	}
	return &etsiPAdES{
		privateKey:            p.PrivateKey,
		certificate:           p.Certificate,
		caCert:                p.CaCert,
		crlDistributionPoints: p.CLRDistributionPoints,
		ocspServers:           p.OCSPServers,
		timestampServerURL:    p.CertificateTimestampServerURL,
	}, handler, nil
}

type etsiPAdES struct {
	privateKey  *rsa.PrivateKey
	certificate *x509.Certificate

	emptySignature bool
	isInitializing bool

	dss                   *model.DSS
	caCert                *x509.Certificate
	crlDistributionPoints []string
	ocspServers           []string
	timestampServerURL    string
}

type padesSignatureHandler interface {
	model.SignatureHandler
	GetDSS() *model.DSS
	SetCACert(*x509.Certificate)
	AddCRLDistributionPoints(...string)
	AddOCSPServers(...string)
	SetTimestampServerURL(string)
}

// NewEmptyPAdES creates a new Adobe.PPKMS/Adobe.PPKLite adbe.pkcs7.detached signature handler.
func NewEmptyPAdES() (padesSignatureHandler, error) {
	return &etsiPAdES{
		emptySignature: true,
	}, nil
}

// GetDSS returns a DSS Pdf dictionary (ETSI TS 102 778-4 V1.1.1).
func (a *etsiPAdES) GetDSS() *model.DSS {
	return a.dss
}

// SetCACert sets the certificate of the root certificate authority.
func (a *etsiPAdES) SetCACert(caCert *x509.Certificate) {
	a.caCert = caCert
}

// AddCRLDistributionPoints adds URLs of CRL distribution points.
func (a *etsiPAdES) AddCRLDistributionPoints(dp ...string) {
	a.crlDistributionPoints = append(a.crlDistributionPoints, dp...)
}

// AddOCSPServers adds URLs of OCSP servers.
func (a *etsiPAdES) AddOCSPServers(servers ...string) {
	a.ocspServers = append(a.ocspServers, servers...)
}

// SetTimestampServerURL sets the URL of a timestamp server.
func (a *etsiPAdES) SetTimestampServerURL(timestampServerURL string) {
	a.timestampServerURL = timestampServerURL
}

// InitSignature initialises the PdfSignature.
func (a *etsiPAdES) InitSignature(sig *model.PdfSignature) error {
	if !a.emptySignature {
		if a.certificate == nil {
			return errors.New("certificate must not be nil")
		}
		if a.privateKey == nil {
			return errors.New("privateKey must not be nil")
		}
	}

	a.dss = new(model.DSS)
	a.dss.VRI = make(map[string]model.DSSCerts)
	handler := *a
	sig.Handler = &handler
	sig.Filter = core.MakeName("Adobe.PPKLite")
	sig.SubFilter = core.MakeName("ETSI.CAdES.detached")
	sig.Reference = nil

	digest, err := handler.NewDigest(sig)
	if err != nil {
		return err
	}
	_, err = digest.Write([]byte("calculate the Contents field size"))
	if err != nil {
		return err
	}
	handler.isInitializing = true
	err = handler.Sign(sig, digest)
	handler.isInitializing = false
	return err
}

func (a *etsiPAdES) makeCRLRequest(server string) ([]byte, error) {
	resp, err := http.Get(server)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (a *etsiPAdES) makeCRLRequests() ([]*core.PdfObjectStream, error) {
	var res []*core.PdfObjectStream
	for _, server := range a.crlDistributionPoints {
		data, err := a.makeCRLRequest(server)
		if err != nil {
			return nil, err
		}
		s, err := core.MakeStream(data, core.NewRawEncoder())
		if err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, nil
}

func (a *etsiPAdES) makeOCSPRequest(server string, cert *x509.Certificate, caCert *x509.Certificate) ([]byte, error) {
	data, err := ocsp.CreateRequest(cert, caCert, &ocsp.RequestOptions{Hash: crypto.SHA1})
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(server, "application/ocsp-request", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ = ioutil.ReadAll(resp.Body)
	_, err = ocsp.ParseResponseForCert(data, nil, caCert)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *etsiPAdES) makeOCSPRequests() ([]*core.PdfObjectStream, error) {
	if a.caCert == nil {
		return nil, nil
	}
	var res []*core.PdfObjectStream
	for _, server := range a.ocspServers {
		data, err := a.makeOCSPRequest(server, a.certificate, a.caCert)
		if err != nil {
			return nil, err
		}
		s, err := core.MakeStream(data, core.NewRawEncoder())
		if err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, nil
}

func (a *etsiPAdES) makeTimestampRequest(server string, encryptedDigest []byte) (asn1.RawValue, error) {
	h := crypto.SHA512.New()
	h.Write(encryptedDigest)
	s := h.Sum(nil)
	r := timestamp.Request{
		HashAlgorithm:   crypto.SHA512,
		HashedMessage:   s,
		Certificates:    true,
		Extensions:      nil,
		ExtraExtensions: nil,
	}
	data, err := r.Marshal()
	if err != nil {
		return asn1.RawValue{}, err
	}

	resp, err := http.Post(server, "application/timestamp-query", bytes.NewBuffer(data))
	if err != nil {
		return asn1.RawValue{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return asn1.RawValue{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return asn1.RawValue{}, fmt.Errorf("http status code not ok (got %d)", resp.StatusCode)
	}

	var ci struct {
		Version asn1.RawValue
		Content asn1.RawValue
	}

	_, err = asn1.Unmarshal(body, &ci)
	if err != nil {
		return asn1.RawValue{}, err
	}

	return ci.Content, nil
}

// Sign sets the Contents fields for the PdfSignature.
func (a *etsiPAdES) Sign(sig *model.PdfSignature, digest model.Hasher) error {

	buffer := digest.(*bytes.Buffer)
	signedData, err := pkcs7.NewSignedData(buffer.Bytes())
	if err != nil {
		return err
	}

	config := pkcs7.SignerInfoConfig{}
	h := crypto.SHA1.New()
	h.Write(a.certificate.Raw)

	var signingCertificate struct {
		Seq struct {
			Seq struct {
				Value []byte
			}
		}
	}

	signingCertificate.Seq.Seq.Value = h.Sum(nil)

	config.ExtraSignedAttributes = append(config.ExtraSignedAttributes, pkcs7.Attribute{
		Type:  asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 12},
		Value: signingCertificate,
	})

	var chain []*x509.Certificate
	if a.caCert != nil {
		chain = []*x509.Certificate{a.caCert}
	}

	if err := signedData.AddSignerChain(a.certificate, a.privateKey, chain, config); err != nil {
		return err
	}

	// Call Detach() is you want to remove content from the signature
	// and generate an S/MIME detached signature
	signedData.Detach()

	if len(a.timestampServerURL) > 0 {
		mDigest := signedData.GetSignedData().SignerInfos[0].EncryptedDigest
		for _, a := range signedData.GetSignedData().SignerInfos[0].AuthenticatedAttributes {
			if a.Type.Equal(pkcs7.OIDAttributeMessageDigest) {
				mDigest = a.Value.Bytes
			}
		}
		tsInfo, err := a.makeTimestampRequest(a.timestampServerURL, mDigest)
		if err != nil {
			return err
		}

		err = signedData.GetSignedData().SignerInfos[0].SetUnauthenticatedAttributes([]pkcs7.Attribute{{
			Type:  asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14},
			Value: tsInfo,
		}})
		if err != nil {
			return err
		}
	}

	// Finish() to obtain the signature bytes
	detachedSignature, err := signedData.Finish()
	if err != nil {
		return err
	}

	data := make([]byte, len(detachedSignature)+1024*2)
	copy(data, detachedSignature)

	sig.Contents = core.MakeHexString(string(data))

	if a.isInitializing {
		return nil
	}

	h = sha1.New()
	h.Write(data)
	key := strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
	stream, err := core.MakeStream(a.certificate.Raw, core.NewRawEncoder())
	if err != nil {
		return err
	}
	a.dss.Certs = append(a.dss.Certs, stream)

	OCSPs, err := a.makeOCSPRequests()
	if err != nil {
		return err
	}
	a.dss.OCSPs = OCSPs

	CLRs, err := a.makeCRLRequests()
	if err != nil {
		return err
	}
	a.dss.CLRs = CLRs
	a.dss.VRI[key] = a.dss.DSSCerts

	return nil
}

// NewDigest creates a new digest.
func (a *etsiPAdES) NewDigest(_ *model.PdfSignature) (model.Hasher, error) {
	return bytes.NewBuffer(nil), nil
}

// Validate validates PdfSignature.
func (a *etsiPAdES) Validate(sig *model.PdfSignature, digest model.Hasher) (model.SignatureValidationResult, error) {
	return a.ValidateEx(sig, digest, nil)
}

// ValidateEx validates PdfSignature with OCSP responses from the DSS dictionary from the PdfReader.
func (a *etsiPAdES) ValidateEx(sig *model.PdfSignature, digest model.Hasher, r *model.PdfReader) (model.SignatureValidationResult, error) {
	signed := sig.Contents.Bytes()

	h := sha1.New()
	h.Write(signed)
	vriKey := strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
	var vri *model.DSSCerts
	if r != nil && r.DSS != nil {
		if v, ok := r.DSS.VRI[vriKey]; ok {
			vri = &v
		}
	}

	p7, err := pkcs7.Parse(signed)
	if err != nil {
		return model.SignatureValidationResult{}, err
	}
	var vriCertificates []*x509.Certificate
	if vri != nil {
		for _, stream := range vri.Certs {
			cert, err := x509.ParseCertificate(stream.Stream)
			if err != nil {
				return model.SignatureValidationResult{}, err
			}
			vriCertificates = append(vriCertificates, cert)
		}
	}

	signer := p7.GetOnlySigner()
	var issuer *x509.Certificate
	for _, cert := range vriCertificates {
		sn := cert.Subject.SerialNumber
		cn := cert.Subject.CommonName
		if sn == signer.Issuer.SerialNumber && cn == signer.Issuer.CommonName {
			issuer = cert
		}
	}

	buffer := digest.(*bytes.Buffer)
	p7.Content = buffer.Bytes()

	if err = p7.Verify(); err != nil {
		return model.SignatureValidationResult{}, err
	}

	result := model.SignatureValidationResult{
		IsSigned:   true,
		IsVerified: true,
		IsVRIFound: vri != nil,
	}

	if vri != nil {
		result.IsOCSPsFound = len(vri.OCSPs) > 0
		for i, stream := range vri.OCSPs {
			if i == 0 {
				result.IsVerifiedByOCSPs = true
			}
			if _, err := ocsp.ParseResponseForCert(stream.Stream, signer, issuer); err != nil {
				result.Errors = append(result.Errors, err.Error())
				result.IsVerifiedByOCSPs = false
			}
		}
		result.IsCLRsFound = len(vri.CLRs) > 0
		for i, stream := range vri.CLRs {
			if i == 0 {
				result.IsVerifiedByCLRs = true
			}
			clr, err := x509.ParseCRL(stream.Stream)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				result.IsVerifiedByCLRs = false
				continue
			}
			for _, rc := range clr.TBSCertList.RevokedCertificates {
				if rc.SerialNumber.Cmp(signer.SerialNumber) == 0 {
					result.Errors = append(result.Errors, fmt.Sprintf("Certificate with SerialNumber %s is revoked", signer.SerialNumber.String()))
					result.IsVerifiedByCLRs = false
				}
				for _, cert := range vriCertificates {
					if rc.SerialNumber.Cmp(cert.SerialNumber) == 0 {
						result.Errors = append(result.Errors, fmt.Sprintf("Certificate with SerialNumber %s is revoked", signer.SerialNumber.String()))
						result.IsVerifiedByCLRs = false
					}
				}
			}
		}
	}

	return result, nil
}

// IsApplicable returns true if the signature handler is applicable for the PdfSignature.
func (a *etsiPAdES) IsApplicable(sig *model.PdfSignature) bool {
	if sig == nil || sig.Filter == nil || sig.SubFilter == nil {
		return false
	}
	return (*sig.Filter == "Adobe.PPKLite") && *sig.SubFilter == "ETSI.CAdES.detached"
}

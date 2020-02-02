package sighandler

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"errors"

	"github.com/gunnsth/pkcs7"
	"github.com/unidoc/unipdf/v3/core"
	"github.com/unidoc/unipdf/v3/model"
)

type etsiPAdES struct {
	privateKey  *rsa.PrivateKey
	certificate *x509.Certificate

	emptySignature    bool
	emptySignatureLen int
}

// NewEmptyEtsiPAdESDetached creates a new Adobe.PPKMS/Adobe.PPKLite adbe.pkcs7.detached
// signature handler. The generated signature is empty and of size signatureLen.
// The signatureLen parameter can be 0 for the signature validation.
func NewEmptyEtsiPAdESDetached(signatureLen int) (model.SignatureHandler, error) {
	return &etsiPAdES{
		emptySignature:    true,
		emptySignatureLen: signatureLen,
	}, nil
}

// NewEtsiPAdESDetached creates a new Adobe.PPKMS/Adobe.PPKLite adbe.pkcs7.detached signature handler.
// Both parameters may be nil for the signature validation.
func NewEtsiPAdESDetached(privateKey *rsa.PrivateKey, certificate *x509.Certificate) (model.SignatureHandler, error) {
	return &etsiPAdES{
		certificate: certificate,
		privateKey:  privateKey,
	}, nil
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

	handler := *a
	sig.Handler = &handler
	sig.Filter = core.MakeName("Adobe.PPKLite")
	sig.SubFilter = core.MakeName("ETSI.CAdES.detached")
	sig.Reference = nil

	digest, err := handler.NewDigest(sig)
	if err != nil {
		return err
	}
	digest.Write([]byte("calculate the Contents field size"))
	return handler.Sign(sig, digest)
}

// Sign sets the Contents fields for the PdfSignature.
func (a *etsiPAdES) Sign(sig *model.PdfSignature, digest model.Hasher) error {
	if a.emptySignature {
		sigLen := a.emptySignatureLen
		if sigLen <= 0 {
			sigLen = 8192
		}

		sig.Contents = core.MakeHexString(string(make([]byte, sigLen)))
		return nil
	}

	buffer := digest.(*bytes.Buffer)
	signedData, err := pkcs7.NewSignedData(buffer.Bytes())
	if err != nil {
		return err
	}

	// Add the signing cert and private key
	if err := signedData.AddSigner(a.certificate, a.privateKey, pkcs7.SignerInfoConfig{}); err != nil {
		return err
	}

	// Call Detach() is you want to remove content from the signature
	// and generate an S/MIME detached signature
	signedData.Detach()
	// Finish() to obtain the signature bytes
	detachedSignature, err := signedData.Finish()
	if err != nil {
		return err
	}

	data := make([]byte, 8192)
	copy(data, detachedSignature)

	sig.Contents = core.MakeHexString(string(data))
	return nil
}

// NewDigest creates a new digest.
func (a *etsiPAdES) NewDigest(sig *model.PdfSignature) (model.Hasher, error) {
	return bytes.NewBuffer(nil), nil
}

// ValidateEx validates PdfSignature with additional information.
func (a *etsiPAdES) Validate(sig *model.PdfSignature, digest model.Hasher) (model.SignatureValidationResult, error) {
	signed := sig.Contents.Bytes()

	p7, err := pkcs7.Parse(signed)
	if err != nil {
		return model.SignatureValidationResult{}, err
	}

	buffer := digest.(*bytes.Buffer)
	p7.Content = buffer.Bytes()

	if err = p7.Verify(); err != nil {
		return model.SignatureValidationResult{}, err
	}

	return model.SignatureValidationResult{
		IsSigned:   true,
		IsVerified: true,
	}, nil
}

// IsApplicable returns true if the signature handler is applicable for the PdfSignature.
func (a *etsiPAdES) IsApplicable(sig *model.PdfSignature) bool {
	if sig == nil || sig.Filter == nil || sig.SubFilter == nil {
		return false
	}
	return (*sig.Filter == "Adobe.PPKLite") && *sig.SubFilter == "ETSI.CAdES.detached"
}

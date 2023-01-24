package transit

import (
	"context"
	"errors"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/keysutil"
	"github.com/hashicorp/vault/sdk/logical"
)

func (b *backend) pathRotate() *framework.Path {
	return &framework.Path{
		Pattern: "keys/" + framework.GenericNameRegex("name") + "/rotate",
		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the key",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.UpdateOperation: b.pathRotateWrite,
		},

		HelpSynopsis:    pathRotateHelpSyn,
		HelpDescription: pathRotateHelpDesc,
	}
}

func (b *backend) pathRotateWrite(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	name := d.Get("name").(string)
	managedKeyName := d.Get("managed_key_name").(string)
	managedKeyId := d.Get("managed_key_id").(string)

	// Get the policy
	p, _, err := b.GetPolicy(ctx, keysutil.PolicyRequest{
		Storage: req.Storage,
		Name:    name,
	}, b.GetRandomReader())
	if err != nil {
		return nil, err
	}
	if p == nil {
		return logical.ErrorResponse("key not found"), logical.ErrInvalidRequest
	}
	if !b.System().CachingDisabled() {
		p.Lock(true)
	}

	var keyId string
	if p.Type == keysutil.KeyType_MANAGED_KEY {
		managedKeySystemView, ok := b.System().(logical.ManagedKeySystemView)
		if !ok {
			return nil, errors.New("unsupported system view")
		}

		keyId, err = keysutil.GetManagedKeyUUID(
			&keysutil.ManagedKeyParameters{
				ManagedKeySystemView: managedKeySystemView,
				BackendUUID:          b.backendUUID,
				Context:              ctx,
			},
			managedKeyName,
			managedKeyId)

		if err != nil {
			return nil, err
		}
	}

	// Rotate the policy
	err = p.Rotate(ctx, req.Storage, b.GetRandomReader(), keyId)

	p.Unlock()
	return nil, err
}

const pathRotateHelpSyn = `Rotate named encryption key`

const pathRotateHelpDesc = `
This path is used to rotate the named key. After rotation,
new encryption requests using this name will use the new key,
but decryption will still be supported for older versions.
`

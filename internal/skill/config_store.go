package skill

import (
	"github.com/neomody77/kuro/internal/credential"
)

// credentialConfigStore adapts credential.Store to the SkillConfigStore interface.
// Each skill's config is stored as a credential with ID "skill_cfg_{name}".
type credentialConfigStore struct {
	store *credential.Store
}

// NewCredentialConfigStore wraps a credential.Store for skill configuration storage.
func NewCredentialConfigStore(store *credential.Store) SkillConfigStore {
	return &credentialConfigStore{store: store}
}

func skillCredID(skillName string) string {
	return "skill_cfg_" + skillName
}

func (s *credentialConfigStore) GetConfig(skillName string) (map[string]string, error) {
	cred, err := s.store.Get(skillCredID(skillName))
	if err != nil {
		return nil, err
	}
	return cred.Data, nil
}

func (s *credentialConfigStore) SaveConfig(skillName string, data map[string]string) error {
	_, err := s.store.Save(credential.Credential{
		ID:   skillCredID(skillName),
		Name: "skill:" + skillName,
		Type: "skill_config",
		Data: data,
	})
	return err
}

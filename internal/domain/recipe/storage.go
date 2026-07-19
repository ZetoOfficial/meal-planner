package recipe

type StorageProfile string

const (
	StorageKeeps3d       StorageProfile = "keeps_3d"
	StorageKeeps3dReheat StorageProfile = "keeps_3d_reheat"
	StorageAssembleFresh StorageProfile = "assemble_fresh"
	StorageEatSameDay    StorageProfile = "eat_same_day"
)

func (p StorageProfile) Valid() bool {
	switch p {
	case StorageKeeps3d, StorageKeeps3dReheat, StorageAssembleFresh, StorageEatSameDay:
		return true
	default:
		return false
	}
}

func (p StorageProfile) SupportsMultiDay() bool {
	switch p {
	case StorageKeeps3d, StorageKeeps3dReheat, StorageAssembleFresh:
		return true
	default:
		return false
	}
}

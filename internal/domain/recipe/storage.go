package recipe

// StorageProfile — как долго блюдо живёт после готовки (hard constraint §1).
type StorageProfile string

const (
	StorageKeeps3d       StorageProfile = "keeps_3d"
	StorageKeeps3dReheat StorageProfile = "keeps_3d_reheat"
	StorageAssembleFresh StorageProfile = "assemble_fresh"
	StorageEatSameDay    StorageProfile = "eat_same_day"
)

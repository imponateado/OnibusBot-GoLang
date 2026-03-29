package domain

type SubscriptionRepository interface {
	Save(subs []UserSubscription) error
	Load() ([]UserSubscription, error)
}

type GroupRepository interface {
	Load() ([]BusGroup, error)
}

type UserRepository interface {
	Save(users []RegisteredUser) error
	Load() ([]RegisteredUser, error)
}

type UserPrefsRepository interface {
	SaveLowMode(lowModeUsers map[int64]bool) error
	LoadLowMode() (map[int64]bool, error)
	SaveBroadcastOptOut(optOutUsers map[int64]bool) error
	LoadBroadcastOptOut() (map[int64]bool, error)
}

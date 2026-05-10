package wizard

func defaultSections(parent *Model) []section {
	return []section{
		newAccountSection(parent),
		newThemeSection(parent),
		newConfirmSection(parent),
	}
}

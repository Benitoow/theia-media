package setup

import (
	"os"
	"strings"
)

// The installer's own words, in French and English.
//
// The interface ships in both languages with French as the default (founding
// spec §3), and that rule does not stop at the browser: the installer is the
// first thing anybody sees of this product, and it is the one screen a person
// reads before anything works. A terminal is not an excuse for English.
//
// The catalogue is a map rather than a struct so that a missing key is visible
// in one place - and TestEverySentenceExistsInBothLanguages fails when one is
// added to a single language.
type Catalogue map[string]string

var french = Catalogue{
	"title":           "Installation de Theia",
	"intro":           "Trois questions, puis c'est fini. Rien n'est écrit avant la confirmation.",
	"roleTitle":       "Cette machine sert-elle, ou regarde-t-elle ?",
	"roleDescription": "Le rôle est déclaré, jamais deviné : un mini-PC dans un placard et un HTPC sous la télé ont la même empreinte.",
	"roleAllInOne":    "Tout-en-un — elle sert et elle regarde",
	"roleServer":      "Serveur seul — elle range et diffuse, sans écran",
	"rolePlayer":      "Lecteur seul — elle regarde la bibliothèque d'une autre machine",

	"pathsTitle":       "Où Theia range-t-il ses données ?",
	"pathsDescription": "Base de données, caches, configuration. Le dossier est créé s'il n'existe pas.",
	"portTitle":        "Port d'écoute",
	"portDescription":  "8383 par défaut. À changer seulement s'il est déjà pris.",
	"hostTitle":        "Nom sur le réseau",
	"hostDescription":  "Les autres appareils joignent cette machine à <nom>.local.",
	"libraryTitle":     "Quels dossiers contiennent vos films ?",
	"libraryHint":      "Un dossier par ligne. Vous pourrez en ajouter plus tard dans les réglages ; laisser vide est permis.",
	"serviceTitle":     "Démarrer Theia automatiquement ?",
	"serviceDesc":      "Une entrée de démarrage est posée sur cette machine, et rien d'autre. Aucun droit administrateur n'est demandé.",
	"confirmTitle":     "Installer avec ces réglages ?",
	"confirmYes":       "Installer",
	"confirmNo":        "Annuler",

	"cancelled": "Annulé : rien n'a été modifié.",
	"done":      "Terminé.",
	"actDir":    "dossier de données créé :",
	"actConfig": "configuration écrite :",
	"actKnown":  "configuration existante conservée :",
	"actServ":   "démarrage automatique installé :",
	"actNone":   "démarrage automatique non demandé.",
	"servFail":  "le démarrage automatique n'a pas pu être installé :",
	"mechanism": "mécanisme",

	"statusTitle":   "État de cette machine",
	"statusRole":    "Rôle",
	"statusData":    "Dossier de données",
	"statusConfig":  "Configuré",
	"statusPort":    "Port",
	"statusHost":    "Nom réseau",
	"statusLibrary": "Dossiers surveillés",
	"statusAuto":    "Démarrage automatique",
	"statusServers": "Binaires trouvés",
	"statusNone":    "aucun",
	"statusYes":     "oui",
	"statusNo":      "non",
	"statusMissing": "absent",

	"updateTitle":     "Mise à jour",
	"updateCurrent":   "Version installée",
	"updateLatest":    "Version publiée",
	"updateAvailable": "Une mise à jour est disponible.",
	"updateNewest":    "Cette installation est à jour.",
	"updateApplied":   "Mise à jour installée. Le serveur la prendra à son prochain démarrage.",
	"updateFailed":    "La mise à jour a échoué :",
	"updateChecking":  "Interrogation de GitHub Releases…",

	"serviceInstalled": "Démarrage automatique installé :",
	"serviceRemoved":   "Démarrage automatique retiré :",
	"serviceNone":      "Aucun démarrage automatique n'était installé.",

	"errorPrefix": "Erreur :",
}

var english = Catalogue{
	"title":           "Installing Theia",
	"intro":           "Three questions, then it is done. Nothing is written before you confirm.",
	"roleTitle":       "Does this machine serve, or watch?",
	"roleDescription": "The role is declared, never guessed: a mini-PC in a cupboard and a home-theatre PC share the same fingerprint.",
	"roleAllInOne":    "All-in-one — it serves and it watches",
	"roleServer":      "Server only — it stores and streams, with no screen",
	"rolePlayer":      "Player only — it watches another machine's library",

	"pathsTitle":       "Where should Theia keep its data?",
	"pathsDescription": "Database, caches, configuration. The folder is created if it is not there.",
	"portTitle":        "Listening port",
	"portDescription":  "8383 unless something else already uses it.",
	"hostTitle":        "Name on the network",
	"hostDescription":  "Other devices reach this machine at <name>.local.",
	"libraryTitle":     "Which folders hold your films?",
	"libraryHint":      "One folder per line. You can add more later in the settings page; leaving this empty is allowed.",
	"serviceTitle":     "Start Theia automatically?",
	"serviceDesc":      "A startup entry is placed on this machine and nothing else. No administrator rights are requested.",
	"confirmTitle":     "Install with these settings?",
	"confirmYes":       "Install",
	"confirmNo":        "Cancel",

	"cancelled": "Cancelled: nothing was changed.",
	"done":      "Done.",
	"actDir":    "data directory created:",
	"actConfig": "configuration written:",
	"actKnown":  "existing configuration kept:",
	"actServ":   "autostart installed:",
	"actNone":   "autostart not requested.",
	"servFail":  "autostart could not be installed:",
	"mechanism": "mechanism",

	"statusTitle":   "This machine",
	"statusRole":    "Role",
	"statusData":    "Data directory",
	"statusConfig":  "Configured",
	"statusPort":    "Port",
	"statusHost":    "Network name",
	"statusLibrary": "Watched folders",
	"statusAuto":    "Autostart",
	"statusServers": "Binaries found",
	"statusNone":    "none",
	"statusYes":     "yes",
	"statusNo":      "no",
	"statusMissing": "missing",

	"updateTitle":     "Update",
	"updateCurrent":   "Installed version",
	"updateLatest":    "Published version",
	"updateAvailable": "An update is available.",
	"updateNewest":    "This installation is up to date.",
	"updateApplied":   "Update installed. The server will use it at its next start.",
	"updateFailed":    "The update failed:",
	"updateChecking":  "Asking GitHub Releases…",

	"serviceInstalled": "Autostart installed:",
	"serviceRemoved":   "Autostart removed:",
	"serviceNone":      "No autostart entry was installed.",

	"errorPrefix": "Error:",
}

// roleLabel is the short name of a role, read off the option it came from: each
// option explains itself after an em dash, and a summary wants only the name.
func roleLabel(role Role, language Catalogue) string {
	key, ok := map[Role]string{
		RoleAllInOne: "roleAllInOne",
		RoleServer:   "roleServer",
		RolePlayer:   "rolePlayer",
	}[role]
	if !ok {
		return string(role)
	}
	label, ok := language[key]
	if !ok {
		return string(role)
	}
	if index := strings.Index(label, " — "); index > 0 {
		return label[:index]
	}
	return label
}

// CatalogueFor picks a language from an explicit choice, then from the locale
// environment, and falls back to French - the product's default, not the
// terminal's guess.
func CatalogueFor(language string) (Catalogue, string) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "fr", "fr-fr", "fra", "french", "français":
		return french, "fr"
	case "en", "en-us", "en-gb", "eng", "english":
		return english, "en"
	case "":
		return fromEnvironment(), fromEnvironmentLanguage()
	default:
		return french, "fr"
	}
}

func fromEnvironment() Catalogue {
	switch fromEnvironmentLanguage() {
	case "en":
		return english
	default:
		return french
	}
}

// fromEnvironmentLanguage reads the locale variables the platforms actually set.
// LANG on macOS and Linux, LC_ALL when somebody is being specific; Windows sets
// neither for a GUI application, so the default holds there.
func fromEnvironmentLanguage() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := strings.ToLower(os.Getenv(name))
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "en") {
			return "en"
		}
		if strings.HasPrefix(value, "fr") {
			return "fr"
		}
	}
	return "fr"
}

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
	"brand":           "THEIA",
	"title":           "Installation de Theia",
	"intro":           "Quelques questions, puis c'est fini. Rien n'est écrit avant la confirmation.",
	"roleTitle":       "Cette machine sert-elle, ou regarde-t-elle ?",
	"roleDescription": "Le rôle est déclaré, jamais deviné : un mini-PC dans un placard et un HTPC sous la télé ont la même empreinte.",
	"roleAllInOne":    "Tout-en-un — elle sert et elle regarde",
	"roleServer":      "Serveur seul — elle range et diffuse, sans écran",
	"rolePlayer":      "Lecteur seul — elle regarde la bibliothèque d'une autre machine",

	// The hint under the form. One line per kind of page, saying what that page
	// actually accepts: enter validates an answer but starts a new line in the
	// folder box, and escape cancels - it did not, when this was Huh's own line
	// built from the bindings.
	"helpSelect":  "↑↓ choisir · entrée valider · / filtrer · échap annuler",
	"helpInput":   "entrée continuer · maj+tab revenir · échap annuler",
	"helpText":    "entrée continuer · alt+entrée nouvelle ligne · échap annuler",
	"helpConfirm": "←→ choisir · y/n · entrée valider · échap annuler",

	"pathsTitle":         "Où Theia range-t-il ses données ?",
	"pathsDescription":   "Base de données, caches, configuration. Le dossier est créé s'il n'existe pas.",
	"portTitle":          "Port d'écoute",
	"portDescription":    "8383 par défaut. À changer seulement s'il est déjà pris.",
	"portInvalid":        "Un port est un nombre entre 1 et 65535.",
	"hostTitle":          "Nom sur le réseau",
	"hostDescription":    "Les autres appareils joignent cette machine à <nom>.local.",
	"hostInvalid":        "Ce nom ne peut pas être vide.",
	"libraryTitle":       "Quels dossiers contiennent vos films ?",
	"libraryHint":        "Un dossier par ligne. Vous pourrez en ajouter plus tard dans les réglages ; laisser vide est permis.",
	"libraryNone":        "aucun pour l'instant",
	"libraryPlaceholder": "D:\\Films",
	"serviceTitle":       "Démarrer Theia automatiquement ?",
	"serviceDesc":        "Une entrée de démarrage est posée sur cette machine, et rien d'autre. Aucun droit administrateur n'est demandé.",
	"confirmTitle":       "Installer avec ces réglages ?",
	"confirmYes":         "Installer",
	"confirmNo":          "Annuler",

	"cancelled":    "Annulé : rien n'a été modifié.",
	"interrupted":  "Interrompu : rien de plus n'a été écrit. Ce qui était déjà installé reste en place.",
	"done":         "Terminé.",
	"actDir":       "dossier de données créé :",
	"actConfig":    "configuration écrite :",
	"actKnown":     "configuration existante conservée :",
	"actServ":      "démarrage automatique installé :",
	"actNone":      "démarrage automatique non demandé.",
	"actProg":      "programme installé :",
	"actShortcut":  "raccourci créé :",
	"shortcutFail": "certains raccourcis n'ont pas pu être créés :",
	"actFetch":     "programme téléchargé :",
	"servFail":     "le démarrage automatique n'a pas pu être installé :",
	"mechanism":    "mécanisme",

	// Installing and removing, as codes: the same list of things on the way in
	// and on the way out, so a person who has just removed Theia reads the same
	// words as the one who installed it.
	"actTool":           "outil d'entretien installé :",
	"actRegistered":     "application inscrite dans la liste des programmes :",
	"actShortcutGone":   "raccourci retiré :",
	"actUnregistered":   "application retirée de la liste des programmes",
	"actServGone":       "démarrage automatique retiré :",
	"actRemoved":        "programmes retirés :",
	"actRemovedLater":   "dossier supprimé dès la fermeture de ce programme :",
	"actRemaining":      "des fichiers n'ont pas pu être retirés :",
	"actNotInstalled":   "aucune installation dans :",
	"uninstallKeptData": "Vos données n'ont pas été touchées :",

	// Where the programs come from, as codes: what a person needs to know is
	// whether the installer copied something already on the machine or fetched
	// it from the release page.
	"originAlreadyInstalled": "déjà en place",
	"originBesideInstaller":  "à côté de l'installeur",
	"originFolder":           "depuis le dossier indiqué",
	"originArchive":          "depuis l'archive indiquée",

	// What the installation is doing while it does it. The programs are named
	// rather than described, so the same sentence works for both of them.
	"shortcutTheiaName":  "Theia",
	"shortcutServerName": "Theia Server",
	"shortcutPlayerName": "Theia Player",
	"shortcutTheia":      "Theia — regarder et diffuser votre bibliothèque",
	"shortcutServer":     "Theia Server — le serveur, dans une fenêtre de journal",
	"shortcutPlayer":     "Theia Player — le lecteur natif",

	"program.server":    "le serveur",
	"program.player":    "le lecteur",
	"progressChecking":  "Vérification de ce qui est publié…",
	"progressDownload":  "Téléchargement : %s",
	"progressExtract":   "Extraction : %s",
	"progressInstall":   "Installation dans %s",
	"progressDone":      "Installation terminée.",
	"progressWaiting":   "Connexion…",
	"progressInterrupt": "échap interrompt",
	"decimalSeparator":  ",",
	"unitMB":            "Mo",
	"unitGB":            "Go",

	"statusTitle":      "État de cette machine",
	"statusRole":       "Rôle",
	"statusData":       "Dossier de données",
	"statusPrograms":   "Programmes",
	"statusRegistered": "Application",
	"statusConfig":     "Configuré",
	"statusPort":       "Port",
	"statusHost":       "Nom réseau",
	"statusLibrary":    "Dossiers surveillés",
	"statusAuto":       "Démarrage automatique",
	"statusServers":    "Binaires trouvés",
	"statusNone":       "aucun",
	"statusYes":        "oui",
	"statusNo":         "non",
	"statusMissing":    "absent",

	"updateTitle":     "Mise à jour",
	"updateCurrent":   "Version installée",
	"updateLatest":    "Version publiée",
	"updateAvailable": "Une mise à jour est disponible.",
	"updateNewest":    "Cette installation est à jour.",
	"updateHeld":      "Rien à installer :",
	"updateApplied":   "Mise à jour installée. Le serveur la prendra à son prochain démarrage.",
	"updateFailed":    "La mise à jour a échoué :",
	"updateChecking":  "Interrogation de GitHub Releases…",

	// The updater answers with codes and the interface owns the sentence
	// (decision 25). A state that is not "available" is not the same as "up to
	// date": a development build refuses to update itself, and saying "up to
	// date" about it would be a lie the user cannot see through.
	"reasonDevelopmentBuild": "c'est une version de développement, elle ne se met pas à jour elle-même.",
	"reasonUpToDate":         "la version publiée est la même.",
	"reasonNoRelease":        "aucune version publiée n'a été trouvée.",
	"reasonGitHub":           "GitHub n'a pas répondu.",
	"reasonNoBinary":         "la version publiée n'a pas de binaire pour cette machine.",
	"reasonOther":            "raison :",

	"serviceInstalled": "Démarrage automatique installé :",
	"serviceRemoved":   "Démarrage automatique retiré :",
	"serviceNone":      "Aucun démarrage automatique n'était installé.",

	// Why an installation stopped. The reasons are codes from internal/release
	// and internal/setup; the sentences are here, because a French machine must
	// never show an English error (decision 25).
	"reasonUnavailable":  "la page des versions n'a pas répondu. Vérifiez la connexion, ou installez depuis l'archive complète.",
	"reasonNotPublished": "cette version ne publie pas %s.",
	"reasonDownload":     "le téléchargement de %s a échoué. Rien n'a été installé.",
	"reasonBundle":       "%s est incomplet : un fichier du paquet manque.",
	"reasonSource":       "%s n'a pas été trouvé là où il était cherché.",

	"errorPrefix": "Erreur :",
}

var english = Catalogue{
	"brand":           "THEIA",
	"title":           "Installing Theia",
	"intro":           "A few questions, then it is done. Nothing is written before you confirm.",
	"roleTitle":       "Does this machine serve, or watch?",
	"roleDescription": "The role is declared, never guessed: a mini-PC in a cupboard and a home-theatre PC share the same fingerprint.",
	"roleAllInOne":    "All-in-one — it serves and it watches",
	"roleServer":      "Server only — it stores and streams, with no screen",
	"rolePlayer":      "Player only — it watches another machine's library",

	"helpSelect":  "↑↓ choose · enter confirm · / filter · esc cancel",
	"helpInput":   "enter continue · shift+tab back · esc cancel",
	"helpText":    "enter continue · alt+enter new line · esc cancel",
	"helpConfirm": "←→ choose · y/n · enter confirm · esc cancel",

	"pathsTitle":         "Where should Theia keep its data?",
	"pathsDescription":   "Database, caches, configuration. The folder is created if it is not there.",
	"portTitle":          "Listening port",
	"portDescription":    "8383 unless something else already uses it.",
	"portInvalid":        "A port is a number between 1 and 65535.",
	"hostTitle":          "Name on the network",
	"hostDescription":    "Other devices reach this machine at <name>.local.",
	"hostInvalid":        "This name cannot be empty.",
	"libraryTitle":       "Which folders hold your films?",
	"libraryHint":        "One folder per line. You can add more later in the settings page; leaving this empty is allowed.",
	"libraryNone":        "none for now",
	"libraryPlaceholder": "D:\\Films",
	"serviceTitle":       "Start Theia automatically?",
	"serviceDesc":        "A startup entry is placed on this machine and nothing else. No administrator rights are requested.",
	"confirmTitle":       "Install with these settings?",
	"confirmYes":         "Install",
	"confirmNo":          "Cancel",

	"cancelled":    "Cancelled: nothing was changed.",
	"interrupted":  "Interrupted: nothing more was written. Whatever was already installed stays.",
	"done":         "Done.",
	"actDir":       "data directory created:",
	"actConfig":    "configuration written:",
	"actKnown":     "existing configuration kept:",
	"actServ":      "autostart installed:",
	"actNone":      "autostart not requested.",
	"actProg":      "program installed:",
	"actShortcut":  "shortcut created:",
	"shortcutFail": "some shortcuts could not be created:",
	"actFetch":     "program downloaded:",
	"servFail":     "autostart could not be installed:",
	"mechanism":    "mechanism",

	"actTool":           "maintenance tool installed:",
	"actRegistered":     "registered in the installed programs list:",
	"actShortcutGone":   "shortcut removed:",
	"actUnregistered":   "removed from the installed programs list",
	"actServGone":       "autostart removed:",
	"actRemoved":        "programs removed:",
	"actRemovedLater":   "folder removed as soon as this program closes:",
	"actRemaining":      "some files could not be removed:",
	"actNotInstalled":   "nothing installed in:",
	"uninstallKeptData": "Your data was not touched:",

	"originAlreadyInstalled": "already in place",
	"originBesideInstaller":  "beside the installer",
	"originFolder":           "from the folder given",
	"originArchive":          "from the archive given",

	"shortcutTheiaName":  "Theia",
	"shortcutServerName": "Theia Server",
	"shortcutPlayerName": "Theia Player",
	"shortcutTheia":      "Theia — watch and stream your library",
	"shortcutServer":     "Theia Server — the server, in a log window",
	"shortcutPlayer":     "Theia Player — the native player",

	"program.server":    "the server",
	"program.player":    "the player",
	"progressChecking":  "Checking what is published…",
	"progressDownload":  "Downloading %s",
	"progressExtract":   "Extracting %s",
	"progressInstall":   "Installing into %s",
	"progressDone":      "Installation finished.",
	"progressWaiting":   "Connecting…",
	"progressInterrupt": "esc interrupts",
	"decimalSeparator":  ".",
	"unitMB":            "MB",
	"unitGB":            "GB",

	"statusTitle":      "This machine",
	"statusRole":       "Role",
	"statusData":       "Data directory",
	"statusPrograms":   "Programs",
	"statusRegistered": "Application",
	"statusConfig":     "Configured",
	"statusPort":       "Port",
	"statusHost":       "Network name",
	"statusLibrary":    "Watched folders",
	"statusAuto":       "Autostart",
	"statusServers":    "Binaries found",
	"statusNone":       "none",
	"statusYes":        "yes",
	"statusNo":         "no",
	"statusMissing":    "missing",

	"updateTitle":     "Update",
	"updateCurrent":   "Installed version",
	"updateLatest":    "Published version",
	"updateAvailable": "An update is available.",
	"updateNewest":    "This installation is up to date.",
	"updateHeld":      "Nothing to install:",
	"updateApplied":   "Update installed. The server will use it at its next start.",
	"updateFailed":    "The update failed:",
	"updateChecking":  "Asking GitHub Releases…",

	"reasonDevelopmentBuild": "this is a development build, and it does not update itself.",
	"reasonUpToDate":         "the published version is the same one.",
	"reasonNoRelease":        "no published release was found.",
	"reasonGitHub":           "GitHub did not answer.",
	"reasonNoBinary":         "the published release has no binary for this machine.",
	"reasonOther":            "reason:",

	"serviceInstalled": "Autostart installed:",
	"serviceRemoved":   "Autostart removed:",
	"serviceNone":      "No autostart entry was installed.",

	"reasonUnavailable":  "the release page did not answer. Check the connection, or install from the complete archive.",
	"reasonNotPublished": "this release does not publish %s.",
	"reasonDownload":     "downloading %s failed. Nothing was installed.",
	"reasonBundle":       "%s is incomplete: a file of the bundle is missing.",
	"reasonSource":       "%s was not found where it was looked for.",

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

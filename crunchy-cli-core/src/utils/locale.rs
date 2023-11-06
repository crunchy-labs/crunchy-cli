use crunchyroll_rs::Locale;

/// Return the locale of the system.
pub fn system_locale() -> Locale {
    if let Some(system_locale) = sys_locale::get_locale() {
        let locale = Locale::from(system_locale);
        if let Locale::Custom(_) = locale {
            Locale::en_US
        } else {
            locale
        }
    } else {
        Locale::en_US
    }
}

/// Check if [`Locale::Custom("all")`] is in the provided locale list and return [`Locale::all`] if
/// so. If not, just return the provided locale list.
pub fn all_locale_in_locales(locales: Vec<Locale>) -> Vec<Locale> {
    if locales
        .iter()
        .any(|l| l.to_string().to_lowercase().trim() == "all")
    {
        Locale::all()
    } else {
        locales
    }
}

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

use crunchyroll_rs::Locale;
use log::warn;

#[derive(Clone, Debug)]
#[allow(clippy::upper_case_acronyms)]
pub enum LanguageTagging {
    Default,
    IETF,
}

impl LanguageTagging {
    pub fn parse(s: &str) -> Result<Self, String> {
        Ok(match s.to_lowercase().as_str() {
            "default" => Self::Default,
            "ietf" => Self::IETF,
            _ => return Err(format!("'{}' is not a valid language tagging", s)),
        })
    }

    pub fn convert_locales(&self, locales: &[Locale]) -> Vec<String> {
        let ietf_language_codes = ietf_language_codes();
        let mut converted = vec![];

        match &self {
            LanguageTagging::Default => {
                for locale in locales {
                    let Some((_, available)) =
                        ietf_language_codes.iter().find(|(_, l)| l.contains(locale))
                    else {
                        // if no matching IETF language code was found, just pass it as it is
                        converted.push(locale.to_string());
                        continue;
                    };
                    converted.push(available.first().unwrap().to_string())
                }
            }
            LanguageTagging::IETF => {
                for locale in locales {
                    let Some((tag, _)) =
                        ietf_language_codes.iter().find(|(_, l)| l.contains(locale))
                    else {
                        // if no matching IETF language code was found, just pass it as it is
                        converted.push(locale.to_string());
                        continue;
                    };
                    converted.push(tag.to_string())
                }
            }
        }

        converted
    }

    pub fn for_locale(&self, locale: &Locale) -> String {
        match &self {
            LanguageTagging::Default => ietf_language_codes()
                .iter()
                .find(|(_, l)| l.contains(locale))
                .map_or(locale.to_string(), |(_, l)| l[0].to_string()),
            LanguageTagging::IETF => ietf_language_codes()
                .iter()
                .find(|(_, l)| l.contains(locale))
                .map_or(locale.to_string(), |(tag, _)| tag.to_string()),
        }
    }
}

pub fn resolve_locales(locales: &[Locale]) -> Vec<Locale> {
    let ietf_language_codes = ietf_language_codes();
    let all_locales = Locale::all();

    let mut resolved = vec![];
    for locale in locales {
        if all_locales.contains(locale) {
            resolved.push(locale.clone())
        } else if let Some((_, resolved_locales)) = ietf_language_codes
            .iter()
            .find(|(tag, _)| tag == &locale.to_string().as_str())
        {
            let (first, alternatives) = resolved_locales.split_first().unwrap();

            resolved.push(first.clone());
            // ignoring `Locale::en_IN` because I think the majority of users which want english
            // audio / subs want the "actual" english version and not the hindi accent dub
            if !alternatives.is_empty() && resolved_locales.first().unwrap() != &Locale::en_IN {
                warn!("Resolving locale '{}' to '{}', but there are some alternatives: {}. If you an alternative instead, please write it completely out instead of '{}'", locale, first, alternatives.iter().map(|l| format!("'{l}'")).collect::<Vec<String>>().join(", "), locale)
            }
        } else {
            resolved.push(locale.clone());
            warn!("Unknown locale '{}'", locale)
        }
    }

    resolved
}

fn ietf_language_codes<'a>() -> Vec<(&'a str, Vec<Locale>)> {
    vec![
        ("ar", vec![Locale::ar_ME, Locale::ar_SA]),
        ("ca", vec![Locale::ca_ES]),
        ("de", vec![Locale::de_DE]),
        ("en", vec![Locale::en_US, Locale::hi_IN]),
        ("es", vec![Locale::es_ES, Locale::es_419, Locale::es_LA]),
        ("fr", vec![Locale::fr_FR]),
        ("hi", vec![Locale::hi_IN]),
        ("id", vec![Locale::id_ID]),
        ("it", vec![Locale::it_IT]),
        ("ja", vec![Locale::ja_JP]),
        ("ko", vec![Locale::ko_KR]),
        ("ms", vec![Locale::ms_MY]),
        ("pl", vec![Locale::pl_PL]),
        ("pt", vec![Locale::pt_PT, Locale::pt_BR]),
        ("ru", vec![Locale::ru_RU]),
        ("ta", vec![Locale::ta_IN]),
        ("te", vec![Locale::te_IN]),
        ("th", vec![Locale::th_TH]),
        ("tr", vec![Locale::tr_TR]),
        ("vi", vec![Locale::vi_VN]),
        ("zh", vec![Locale::zh_CN, Locale::zh_HK, Locale::zh_TW]),
    ]
}

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

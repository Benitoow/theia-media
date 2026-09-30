//! Command compatibility for the replaceable engine, including Ubuntu's mpv.
//! mpv 0.38 inserted a playlist index before loadfile's options argument.

pub fn loadfile_arguments<'a>(
    version: &str,
    url: &'a str,
    options: Option<&'a str>,
) -> Vec<&'a str> {
    let generation = version
        .split(|c: char| !c.is_ascii_digit() && c != '.')
        .find(|part| part.contains('.'))
        .and_then(|part| {
            let mut parts = part.split('.');
            Some((
                parts.next()?.parse::<u32>().ok()?,
                parts.next()?.parse::<u32>().ok()?,
            ))
        });
    let indexed = generation.map_or(true, |(major, minor)| major > 0 || minor >= 38);
    let mut arguments = vec!["loadfile", url, "replace"];
    if indexed {
        arguments.push("0");
    }
    if let Some(options) = options {
        arguments.push(options);
    }
    arguments
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ubuntu_resume_options_are_not_interpreted_as_a_playlist_index() {
        assert_eq!(
            loadfile_arguments("mpv 0.37.0", "a film.mkv", Some("start=12,aid=2")),
            ["loadfile", "a film.mkv", "replace", "start=12,aid=2"]
        );
        assert_eq!(
            loadfile_arguments("mpv 0.37.0", "a film.mkv", None),
            ["loadfile", "a film.mkv", "replace"]
        );
    }

    #[test]
    fn newer_and_development_engines_keep_the_index_argument() {
        for version in ["mpv v0.38.0", "mpv v0.41.0-dev-gd97d68da9", "mpv 1.0.0"] {
            assert_eq!(
                loadfile_arguments(version, "film.mkv", Some("start=12")),
                ["loadfile", "film.mkv", "replace", "0", "start=12"]
            );
        }
    }
}

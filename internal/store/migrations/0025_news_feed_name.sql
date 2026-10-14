-- The news feed that GDash adds on the first start was named after its host,
-- feeds.nos.nl. Now that a News widget shows the name of its one source as
-- its title, and widgets choose their sources by name, it gets the name the
-- news catalogue gives it (internal/news/catalogue.json). Only that feed, and
-- only while it still has the name it was given: a name the admin chose is
-- kept.
--
-- Rollback: restore gdash.db.bak-v24, which GDash writes before applying
-- this migration.
UPDATE sources SET name = 'NOS Nieuws'
WHERE kind = 'news' AND address = 'https://feeds.nos.nl/nosnieuwsalgemeen' AND name = 'feeds.nos.nl';

-- Remote access asks the router instead of asking the owner.
--
-- The M4 panel shipped with a UDP port field, a public endpoint field, and a
-- paragraph explaining how to forward a port on a router Theia has never seen.
-- That is four steps in somebody else's admin interface for a product whose
-- founding promise is no configuration. The port and the address are facts the
-- gateway already holds; UPnP IGD and NAT-PMP are how it says them, and both
-- speak only to the local network -- no relay, no control plane, so decision 43
-- is untouched.
--
-- automatic defaults to 1: an existing installation that was set up by hand
-- keeps working, because the manual endpoint it already stored is used whenever
-- discovery declines to answer, and the columns below are only ever filled in
-- by a successful discovery.
ALTER TABLE remote_access_config
    ADD COLUMN automatic INTEGER NOT NULL DEFAULT 1 CHECK (automatic IN (0, 1));

-- What the router last said, kept so the panel can name the method that worked
-- and so a mapping can be withdrawn on the way out.
ALTER TABLE remote_access_config ADD COLUMN mapped_method TEXT NOT NULL DEFAULT '';
ALTER TABLE remote_access_config ADD COLUMN mapped_port   INTEGER NOT NULL DEFAULT 0;

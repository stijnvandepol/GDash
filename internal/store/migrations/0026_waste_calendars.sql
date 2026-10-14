-- What a calendar is for: empty for appointments, which the Calendar
-- widget shows, or 'waste' for a waste collection calendar, which only the
-- Waste collection widget shows and which the Integrations page lists under
-- its own card. News feeds keep an empty purpose. A calendar that a Waste
-- collection widget already names becomes a waste calendar, so the widget
-- keeps working and its pickups leave the Calendar widget.
--
-- Rollback: restore gdash.db.bak-v25, which GDash writes before applying
-- this migration.
ALTER TABLE sources ADD COLUMN purpose TEXT NOT NULL DEFAULT '' CHECK (purpose IN ('', 'waste'));

UPDATE sources SET purpose = 'waste'
WHERE kind = 'calendar' AND id IN (
    SELECT json_extract(widget.value, '$.settings.calendar')
    FROM dashboards, json_each(dashboards.widgets) AS widget
    WHERE json_extract(widget.value, '$.type') = 'waste'
);

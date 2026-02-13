UPDATE link_tasks
SET status = 'searching'
WHERE status = 'found';

UPDATE domains
SET link_status = 'searching'
WHERE link_status = 'found';

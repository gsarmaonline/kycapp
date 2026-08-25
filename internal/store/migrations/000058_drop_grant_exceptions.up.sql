-- The exception lists go.
--
-- Every axis with a wildcard carried one, and the reasoning was sound while a
-- grant was a row a merchant's backend interpreted: a wildcard names a set
-- nobody can enumerate, so no such claim is ever exactly right, and the
-- exception was how you said "everything except that".
--
-- They do not survive the move to edges, and not for want of effort. An
-- exception narrows the grant it sits on and nothing else, which is what kept
-- grants unordered. Expressed in the graph it would have to be a subtraction in
-- a rule, and a rule belongs to a *type*, not to a grant: it would veto every
-- other grant reaching that type, not just the one it was written on. That is
-- precisely the veto invariant 6 forbids, and the thing that brings back
-- ordering, precedence and conflict resolution.
--
-- Generating one rule per grant was the alternative, and it makes the schema a
-- function of the data and unbounded. A deny edge was the other, and the reach
-- package exists to avoid exactly that.
--
-- So the model keeps the answer it always had for a hard lock: grant nothing
-- that reaches the resource. That answer is now the only one, and the UI has to
-- say so rather than let a field quietly disappear.
--
-- except_app_user_ids is the one that was doing real work. It was applied as a
-- SQL NOT ... = ANY on the read path, which has no edge equivalent at all. A
-- merchant relying on it should carve the excluded customers out of the group
-- the grant names instead, which says the same thing with membership.

ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_capability_exceptions_need_wildcard;

ALTER TABLE app_grants DROP COLUMN IF EXISTS except_capabilities;

ALTER TABLE app_grants DROP COLUMN IF EXISTS except_scopes;

ALTER TABLE app_grants DROP COLUMN IF EXISTS except_app_user_ids;

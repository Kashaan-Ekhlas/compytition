-- Onboarding gained a separate location step, so it now has seven steps.

ALTER TABLE users
    DROP CONSTRAINT users_onboarding_step_check,
    ADD CONSTRAINT users_onboarding_step_check
        CHECK (onboarding_step BETWEEN 1 AND 7);

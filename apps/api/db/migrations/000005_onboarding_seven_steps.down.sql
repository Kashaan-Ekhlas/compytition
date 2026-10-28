UPDATE users SET onboarding_step = 6 WHERE onboarding_step = 7;

ALTER TABLE users
    DROP CONSTRAINT users_onboarding_step_check,
    ADD CONSTRAINT users_onboarding_step_check
        CHECK (onboarding_step BETWEEN 1 AND 6);

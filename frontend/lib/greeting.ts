// Maps an hour of the day (0-23) to a greeting. Kept free of Date and React
// so it can be unit tested with plain numbers.
export function getGreeting(hour: number): string {
    if (hour >= 5 && hour < 12) {
        return "Good morning";
    }

    if (hour >= 12 && hour < 18) {
        return "Good afternoon";
    }

    return "Good evening";
}

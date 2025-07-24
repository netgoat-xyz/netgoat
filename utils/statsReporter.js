// statsReporter.js

/**
 * @fileoverview A "smol" Node.js library to collect process-specific and system statistics
 * and send them to a custom server every minute, with an initial authentication step.
 */

const os = require('os'); // Node.js built-in OS module for system info
const process = require('process'); // Node.js built-in process module for process info
/**
 * Global variable to store the interval ID, allowing us to stop reporting later.
 * @type {NodeJS.Timeout | null}
 */
let reportingInterval = null;

/**
 * Stores the dynamically obtained secret key from the server.
 * @type {string | null}
 */
let currentSecretKey = null;

// Variables for app-specific CPU usage calculation
let previousAppCpuUsage = process.cpuUsage();
let previousAppHrTime = process.hrtime.bigint();

/**
 * Calculates the current process's CPU usage percentage over a short interval.
 * This function takes two samples of `process.cpuUsage()` and `process.hrtime.bigint()`
 * to determine the CPU time consumed by the current Node.js process relative to
 * the elapsed wall-clock time.
 * @returns {Promise<number>} A promise that resolves with the process CPU usage percentage.
 */
async function getAppCpuUsage() {
    // Wait for a very short period to get a second sample for calculation.
    // This duration affects the responsiveness and precision of the CPU measurement.
    await new Promise(resolve => setTimeout(resolve, 50)); // Sample over 50ms

    const currentAppCpuUsage = process.cpuUsage();
    const currentAppHrTime = process.hrtime.bigint();

    // Calculate the difference in CPU times (user and system) since the last sample.
    // These are in microseconds.
    const userDiff = currentAppCpuUsage.user - previousAppCpuUsage.user;
    const systemDiff = currentAppCpuUsage.system - previousAppCpuUsage.system;

    // Calculate the difference in high-resolution real time (wall-clock time)
    // since the last sample. This is in nanoseconds.
    const totalHrTimeDiffNs = currentAppHrTime - previousAppHrTime;

    // Update the previous values for the next calculation.
    previousAppCpuUsage = currentAppCpuUsage;
    previousAppHrTime = currentAppHrTime;

    // Total CPU time consumed by the process during the interval (in microseconds).
    const processCpuTimeUs = userDiff + systemDiff;

    // Total elapsed wall-clock time during the interval (in microseconds).
    // Convert nanoseconds to microseconds by dividing by 1000.
    const wallClockTimeUs = Number(totalHrTimeDiffNs / 1000n);

    // Avoid division by zero if the wall-clock time difference is negligible.
    if (wallClockTimeUs === 0) {
        return 0;
    }

    // Calculate CPU usage percentage: (process CPU time / wall-clock time) * 100.
    // This value can sometimes exceed 100% on multi-core systems if the process
    // utilizes multiple cores heavily within the sampling window.
    const cpuUsagePercent = (processCpuTimeUs / wallClockTimeUs) * 100;

    // Return the percentage, formatted to two decimal places.
    return parseFloat(cpuUsagePercent.toFixed(2));
}

/**
 * Collects various system and process-specific statistics.
 * @returns {Promise<Object>} An object containing system and app-specific statistics.
 */
async function collectSystemAndAppStats() {
    // System-wide memory statistics
    const totalMemory = os.totalmem(); // Total system memory in bytes
    const freeMemory = os.freemem();   // Free system memory in bytes
    const usedSystemMemory = totalMemory - freeMemory;

    // Process-specific memory statistics (Resident Set Size - RSS)
    // RSS is the portion of memory occupied by a process that is held in main memory (RAM).
    const appMemoryUsage = process.memoryUsage();
    const appRssMemoryBytes = appMemoryUsage.rss; // Resident Set Size in bytes

    return {
        timestamp: new Date().toISOString(),
        hostname: os.hostname(),
        platform: os.platform(),
        arch: os.arch(),
        systemUptimeSeconds: os.uptime(), // System uptime in seconds

        // System-wide CPU information (model, count, load average)
        cpuCount: os.cpus().length,
        cpuModel: os.cpus()[0].model,
        systemLoadAverage: os.loadavg(), // 1, 5, and 15 minute load averages

        // System-wide memory usage
        totalSystemMemoryBytes: totalMemory,
        freeSystemMemoryBytes: freeMemory,
        usedSystemMemoryBytes: usedSystemMemory,
        systemMemoryUsagePercent: parseFloat(((usedSystemMemory / totalMemory) * 100).toFixed(2)),

        // Process-specific statistics
        processId: process.pid,
        processUptimeSeconds: process.uptime(), // Process uptime in seconds
        appCpuUsagePercent: await getAppCpuUsage(), // Calculated app CPU usage
        appMemoryRssBytes: appRssMemoryBytes, // App's Resident Set Size (RAM usage)
        // Note: Node.js's built-in modules do not provide direct, cross-platform
        // process-specific disk read/write usage. This typically requires
        // platform-specific tools or native modules, which are outside the scope
        // of a "smol" and portable library.
    };
}

/**
 * Decodes the payload of a JWT. This is a simple base64 decoding and
 * does NOT verify the JWT signature. It assumes the server sends a
 * valid, unverified JWT or a base64-encoded JSON in the payload part.
 * For production, consider using a proper JWT library like 'jsonwebtoken'
 * to verify signatures.
 * @param {string} token - The JWT string.
 * @returns {Object | null} The decoded payload object, or null if decoding fails.
 */
function decodeJwtPayload(token) {
    try {
        const parts = token.split('.');
        if (parts.length !== 3) {
            global.logger.error('[StatsReporter] Invalid JWT format: Expected 3 parts.');
            return null;
        }
        const payloadBase64 = parts[1];
        // Decode base64url to base64, then base64 to UTF-8 string
        const decodedPayload = Buffer.from(payloadBase64, 'base64').toString('utf8');
        return JSON.parse(decodedPayload);
    } catch (error) {
        global.logger.error(`[StatsReporter] Error decoding JWT payload: ${error.message}`);
        return null;
    }
}

/**
 * Performs an initial authentication step to obtain a dynamic secret key from the server.
 * This key is then used for subsequent stats reporting requests.
 * @param {string} serverUrl - The base URL of the custom server.
 * @param {string} sharedJwt - The pre-shared JWT for initial authentication.
 * @param {string} service - The service identifier.
 * @param {string} workerId - The worker ID.
 * @param {string} regionId - The region ID.
 * @returns {Promise<string | null>} A promise that resolves with the SecretKey if successful, otherwise null.
 */
async function authenticateAndGetSecretKey(serverUrl, sharedJwt, service, workerId, regionId) {
    const authEndpoint = `${serverUrl}/auth`; // Assuming an /auth endpoint for registration
    global.logger.info(`[StatsReporter] Attempting to authenticate and get SecretKey from ${authEndpoint}...`);

    try {
        const response = await fetch(authEndpoint, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
                'Authorization': `Bearer ${sharedJwt}` // Use the SHARED_JWT for initial authentication
            },
            body: JSON.stringify({
                service: service,
                workerId: workerId,
                regionId: regionId
            }),
        });

        if (!response.ok) {
            const errorText = await response.text();
            global.logger.error(`[StatsReporter] Authentication failed: ${response.status} ${response.statusText} - ${errorText}`);
            return null;
        }

        const responseJson = await response.json();
        // The server is expected to respond with a JWT containing the SecretKey
        const responseJwt = responseJson.token; // Assuming the server sends a 'token' field
        if (!responseJwt) {
            global.logger.error('[StatsReporter] Authentication response missing JWT token.');
            return null;
        }

        const decodedPayload = decodeJwtPayload(responseJwt);
        if (decodedPayload && decodedPayload.secretKey) {
            global.logger.success('[StatsReporter] Successfully obtained SecretKey.');
            return decodedPayload.secretKey;
        } else {
            global.logger.error('[StatsReporter] JWT payload missing "secretKey" field.');
            return null;
        }

    } catch (error) {
        global.logger.error(`[StatsReporter] Error during authentication request to ${authEndpoint}:`, error.message);
        return null;
    }
}

/**
 * Sends the collected data to the specified server URL.
 * Includes the dynamically obtained SecretKey in the headers for authorization.
 * @param {string} serverUrl - The URL of the custom server endpoint.
 * @param {Object} data - The data object to send.
 * @param {string | null} secretKey - The dynamic secret key obtained from authentication.
 */
async function sendDataToServer(serverUrl, data, secretKey) {
    if (!secretKey) {
        global.logger.error('[StatsReporter] Cannot send data: SecretKey is not available. Authentication might have failed.');
        return;
    }

    try {
        const response = await fetch(serverUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
                'X-Secret-Key': secretKey // Include the dynamic SecretKey in a custom header
            },
            body: JSON.stringify(data),
            // You might want to add a timeout for the request
            // signal: AbortSignal.timeout(5000) // Requires Node.js v15.0.0+
        });

        if (!response.ok) {
            // global.logger non-2xx responses as errors
            const errorText = await response.text();
            global.logger.error(`[StatsReporter] Failed to send data: ${response.status} ${response.statusText} - ${errorText}`);
        } else {
            global.logger.info(`[StatsReporter] Data sent successfully to ${serverUrl}`);
        }
    } catch (error) {
        global.logger.error(`[StatsReporter] Error sending data to ${serverUrl}:`, error.message);
    }
}

/**
 * Starts the statistics reporting.
 * @param {Object} options - Configuration options for the reporter.
 * @param {string} options.serverUrl - The base URL of the custom server.
 * It expects /auth for initial handshake and
 * /report-stats for sending data.
 * @param {string} options.sharedJwt - The pre-shared JWT for initial authentication.
 * @param {number} [options.intervalMinutes=1] - The interval in minutes to send data.
 * @param {string} [options.service='default_service'] - The service identifier.
 * @param {string} [options.string='default_worker'] - The worker ID. If not provided,
 * the data key will be based on service and region only.
 * @param {string} [options.regionId='default_region'] - The region ID.
 */
async function startReporting({
    serverUrl,
    sharedJwt,
    intervalMinutes = 1,
    service,
    workerId, 
    regionId
}) {
    if (!serverUrl) {
        global.logger.error('[StatsReporter] Error: serverUrl is required to start reporting.');
        return;
    }
    if (!sharedJwt) {
        global.logger.error('[StatsReporter] Error: sharedJwt is required for authentication.');
        return;
    }

    if (reportingInterval) {
        global.logger.warn('[StatsReporter] Reporting is already running. Stopping previous interval before starting a new one.');
        stopReporting();
    }

    // --- Initial Authentication Step ---
    currentSecretKey = await authenticateAndGetSecretKey(serverUrl, sharedJwt, service, workerId, regionId);

    if (!currentSecretKey) {
        global.logger.error('[StatsReporter] Failed to obtain SecretKey. Reporting will not start.');
        return;
    }

    const intervalMs = intervalMinutes * 60 * 1000; // Convert minutes to milliseconds

    global.logger.info(`[StatsReporter] Starting reporting to ${serverUrl}/report-stats every ${intervalMinutes} minute(s).`);
    const dataKeyParts = [service, regionId];
    if (workerId && workerId !== 'default_worker') { // Only add workerId if it's explicitly provided and not default
        dataKeyParts.push(workerId);
    }
    const dataKey = dataKeyParts.join('_');
    global.logger.info(`[StatsReporter] Data key will be: ${dataKey}`);

    /**
     * The main reporting function that runs on an interval.
     */
    const reportFunction = async () => {
        try {
            const stats = await collectSystemAndAppStats();
            const payload = {
                dataKey: dataKey,
                service: service,
                workerId: workerId,
                regionId: regionId,
                stats: stats,
            };

            global.logger.debug(`[StatsReporter] Collecting and sending stats for ${dataKey}...`);
            await sendDataToServer(`${serverUrl}/report-stats`, payload, currentSecretKey);
        } catch (error) {
            global.logger.error('[StatsReporter] Error during reporting cycle:', error.message);
        }
    };

    // Run immediately on start
    reportFunction();

    // Set up the interval
    reportingInterval = setInterval(reportFunction, intervalMs);
}

/**
 * Stops the statistics reporting.
 */
function stopReporting() {
    if (reportingInterval) {
        clearInterval(reportingInterval);
        reportingInterval = null;
        currentSecretKey = null; // Clear the secret key on stop
        global.logger.info('[StatsReporter] Statistics reporting stopped.');
    } else {
        global.logger.info('[StatsReporter] Statistics reporting is not currently running.');
    }
}

// Export the functions to be used by other modules
module.exports = {
    startReporting,
    stopReporting,
};

import http from 'k6/http';
import {check} from 'k6';

export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '1m'
    },
    load: {
      startTime: '1m',
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        {duration: '1m', target: 100}, // simulate ramp-up of traffic from 1 to 100 users over 2 minutes.
        {duration: '3m', target: 100}, // stay at 100 users for 5 minutes
        {duration: '1m', target: 0}, // ramp-down to 0 users
      ],
    },
    stress: {
      startTime: '6m',
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        {duration: '10s', target: 100}, // simulate ramp-up of traffic from 1 to 100 users over 10 seconds.
        {duration: '1m', target: 100}, // stay at 100 users for 1 minute
        {duration: '10s', target: 1400}, // spike to 1400 users
        {duration: '3m', target: 1400}, // stay at 1400 for 3 minutes
        {duration: '10s', target: 10}, // scale down. Recovery stage.
        {duration: '1m', target: 10},
        {duration: '1m', target: 0}, // Wait a bit more in case requests are still pending
      ]
    }
  },
  thresholds: {
    http_req_failed: ['rate<0.01'], // http errors should be less than 1%
  },

  insecureSkipTLSVerify: true
};

export default function () {
  const url = `${__ENV.API_BASE_URL}/v1/check/hash`;

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    timeout: '300s'
  };

  const responses = http.batch([
    // password
    ['POST', url,
      JSON.stringify({'hash': '5baa61e4c9b93f3f0682250b6cf8331b7ee68fd8'}),
      params],
    // 9Uy34f#qM2zr
    ['POST', url,
      JSON.stringify({'hash': '022fa8ea463319b08304464dc7f7460acba58d34'}),
      params],
    // i love dogs
    ['POST', url,
      JSON.stringify({'hash': 'c524a39c02f142ba0b81da289f2e11332d59b4dd'}),
      params],
  ]);

  check(responses[0], {
    'is ok': (r) => r.status === 200,
    'is pwned': (r) => {
      if (r !== undefined) {
        return JSON.parse(r.body)['pwned'] === true;
      }

      // This means the request timed out or failed.
      return false;
    }
  });

  check(responses[1], {
    'is ok': (r) => r.status === 200,
    'is not pwned': (r) => {
      if (r !== undefined) {
        return JSON.parse(r.body)['pwned'] === false;
      }

      // This means the request timed out or failed.
      return false;
    }
  });

  check(responses[2], {
    'is ok': (r) => r.status === 200,
    'is pwned': (r) => {
      if (r !== undefined) {
        return JSON.parse(r.body)['pwned'] === true;
      }

      // This means the request timed out or failed.
      return false;
    }
  });
}

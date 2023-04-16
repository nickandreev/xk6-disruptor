import { PodDisruptor } from 'k6/x/disruptor';
import http from 'k6/http';


export function disrupt(data) {
    if (__ENV.SKIP_FAULTS == "1") {
        return
    }

    const selector = {
        namespace: "httpbin",
        select: {
            labels: {
                app: "httpbin"
            }
        }
    }
    const podDisruptor = new PodDisruptor(selector)

    // delay traffic from one random replica of the deployment
    const fault = {
        averageDelay: 50,
        errorCode: 500,
        errorRate: 0.1,
	port: 90
    }
    podDisruptor.injectHTTPFaults(fault, 30)
}

export const options = {
    scenarios: {
       disrupt: {
            executor: 'shared-iterations',
            iterations: 1,
            vus: 1,
            exec: "disrupt",
            startTime: "0s",
        },
    }
}

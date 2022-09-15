<p>Packages:</p>
<ul>
<li>
<a href="#logging.extensions.config.gardener.cloud%2fv1alpha1">logging.extensions.config.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="logging.extensions.config.gardener.cloud/v1alpha1">logging.extensions.config.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains theLogging extension configuration.</p>
</p>
Resource Types:
<ul><li>
<a href="#logging.extensions.config.gardener.cloud/v1alpha1.Configuration">Configuration</a>
</li></ul>
<h3 id="logging.extensions.config.gardener.cloud/v1alpha1.Configuration">Configuration
</h3>
<p>
<p>Configuration contains information about the Logging service configuration.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
logging.extensions.config.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Configuration</code></td>
</tr>
<tr>
<td>
<code>healthCheckConfig</code></br>
<em>
<a href="https://github.com/gardener/gardener/extensions/pkg/apis/config">
github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1.HealthCheckConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HealthCheckConfig is the config for the health check controller.</p>
</td>
</tr>
<tr>
<td>
<code>shootPurposesWithNodeLogging</code></br>
<em>
[]string
</em>
</td>
<td>
<p>ShootPurposesWithNodeLogging are the shoot purposes for which there will be installed node logging.</p>
</td>
</tr>
<tr>
<td>
<code>featureGates</code></br>
<em>
map[string]bool
</em>
</td>
<td>
<p>FeatureGates is a map of feature names to bools that enable features.
Default: nil</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>

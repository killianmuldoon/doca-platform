How to use this template:
- Create a new issue with this template as description
- Select which kind of artefact you're publishing - delete irrelevant parts of the issue
- Title the issue `release: $ARTEFACT_NAME for $COMPONENT_NAME` e.g. `release: Helm chart for DPF Operator`
- Track tasks that need to be done for the component release


#### Repository

* [ ] SWIPAT Approval
* [ ] Request creation of a GitHub repo under github.com/NVIDIA
    * [ ] Add DPF team members to the repo
* [ ] Passing score (1000) on Nspect
    * [ ] Add to checkmarx static analysis and get a passing score ([here](https://checkmarx.nvidia.com/cxwebclient/ProjectStateSummary.aspx?projectid=1926))
    * [ ] Add to SBOM
* [ ] Push code to the newly created repo

#### Container

* [ ] SWIPAT approval with[ NVBug](https://nvbugspro.nvidia.com/bug/2885977)
* [ ] Ensure the container has the [correct licensing requirements](https://docs.google.com/document/d/1zY1k8J3X7DYiP6qMTodPmHO9FBBoqHJwatNG6vxnYlc/edit#heading=h.phhp2u63hoot).
* [ ] Add a container to the staging repo
    * [ ] Push container to [staging repository](http://nvcr.io/nvstaging/doca)
    * [ ] Add [name, description and metadata](https://docs.google.com/document/d/1DFMMg6pD-WgbQQHWef45bGfJfagyzgX8VW9DIppIXfo/edit#heading=h.jfka9wvsqabm)
    * [ ] Pass pulse scanning done by NGC
* [ ] Add container to the Nspect project
* [ ] Pass QA testing and submit a report
* [ ] File [NVBug for promotion](https://nvbugspro.nvidia.com/bug/2763936)
* [ ] Promote container to NGC

#### Helm chart

* [ ] SWIPAT approval with [NVBug](https://nvbugspro.nvidia.com/bug/2885977)
* [ ] Add helm chart to the staging repo
    * [ ] Push chart to [staging repository](http://nvcr.io/nvstaging/doca)
    * [ ] Add [name, description and metadata](https://docs.google.com/document/d/1DFMMg6pD-WgbQQHWef45bGfJfagyzgX8VW9DIppIXfo/edit#heading=h.jfka9wvsqabm)
* [ ] Pass QA testing and submit a report
* [ ] [File NVBug for promotion](https://nvbugspro.nvidia.com/bug/2763936)
* [ ] Promote helm chart to NGC
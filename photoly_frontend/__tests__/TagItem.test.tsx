import {describe, expect***REMOVED*** from "@jest/globals";
import {render***REMOVED*** from "@testing-library/react";
import TagItem from "../components/TagItem";


describe("<TagItem/> renders correctly", () => {
    test("should render tag item on the screen", () => {
        const {container, getByText***REMOVED*** = render(<TagItem tagName={"something"***REMOVED***/>)
        expect(container.firstChild).toMatchSnapshot()
        const tag = getByText(/something/i)
        expect(tag).toBeTruthy()
***REMOVED***)
***REMOVED***)
import {describe, expect} from "@jest/globals";
import {render} from "@testing-library/react";
import TagItem from "../components/TagItem";


describe("<TagItem/> renders correctly", () => {
    test("should render tag item on the screen", () => {
        const {container, getByText} = render(<TagItem tagId={1} tagName={"something"}/>)
        expect(container.firstChild).toMatchSnapshot()
        const tag = getByText(/something/i)
        expect(tag).toBeTruthy()
    })
})
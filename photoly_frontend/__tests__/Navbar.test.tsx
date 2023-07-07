import React from "react";
import { render } from "@testing-library/react"
import { useRouter } from 'next/router'

import Navbar from "../components/Navbar";
import {describe, expect} from "@jest/globals";
import userEvent from "@testing-library/user-event";

// useRouter now returns a mock function in this test file
jest.mock('next/router', () => ({
    __esModule: true,
    useRouter: jest.fn(() => {return {
        push: jest.fn() // the component uses `router.push` only
    }}),
}))

describe("<Navbar/> rendering", () => {
    test("should render navigation bar on the screen", async () => {
        const {container, getByText, getByTestId} = render(<Navbar/>);
        expect(container.firstChild).toMatchSnapshot()
        expect(getByText("PHOTOLY")).toBeTruthy()
        expect(getByText("Logout")).toBeTruthy()
        expect(getByText("Manage")).toBeTruthy()
        expect(getByTestId("logo")).toBeTruthy()
        expect(getByTestId("avatar")).toBeTruthy()
    })
})


describe("<Navbar/> functionality", () => {
    test("test manage page redirection", async () => {
        const mockRouter = {
            push: jest.fn() // the component uses `router.push` only
        };
        // make useRouter return an object with a fake push function in it
        (useRouter as jest.Mock).mockReturnValue(mockRouter)
        const {getByText} = render(<Navbar/>);

        await userEvent.click(getByText(/manage/i))
        expect(mockRouter.push).toHaveBeenCalledWith("/manage")
    })

    test("test logout page redirection", async () => {
        const mockRouter = {
            push: jest.fn() // the component uses `router.push` only
        };
        // make useRouter return an object with a fake push function in it
        (useRouter as jest.Mock).mockReturnValue(mockRouter)
        const {getByText} = render(<Navbar/>);

        await userEvent.click(getByText(/logout/i))
        expect(mockRouter.push).toHaveBeenCalledWith("/login")
    })
})

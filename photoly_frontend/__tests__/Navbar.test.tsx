import React from "react";
import {getByTestId, render***REMOVED*** from "@testing-library/react"
import { useRouter ***REMOVED*** from 'next/router'

import Navbar from "../components/Navbar";
import {describe, expect***REMOVED*** from "@jest/globals";
import userEvent from "@testing-library/user-event";

// useRouter now returns a mock function in this test file
jest.mock('next/router', () => ({
    __esModule: true,
    useRouter: jest.fn(() => {return {
        push: jest.fn() // the component uses `router.push` only
***REMOVED******REMOVED***),
***REMOVED***))

describe("<Navbar/> rendering", () => {
    test("should render navigation bar on the screen", async () => {
        const {container, getByText, getByTestId***REMOVED*** = render(<Navbar/>);
        expect(container.firstChild).toMatchSnapshot()
        expect(getByText("PHOTOLY")).toBeTruthy()
        expect(getByText("Logout")).toBeTruthy()
        expect(getByText("Manage")).toBeTruthy()
        expect(getByTestId("logo")).toBeTruthy()
        expect(getByTestId("avatar")).toBeTruthy()
***REMOVED***)
***REMOVED***)


describe("<Navbar/> functionality", () => {
    test("test manage page redirection", async () => {
        const mockRouter = {
            push: jest.fn() // the component uses `router.push` only
***REMOVED***;
        // make useRouter return an object with a fake push function in it
        (useRouter as jest.Mock).mockReturnValue(mockRouter)
        const {getByText***REMOVED*** = render(<Navbar/>);

        await userEvent.click(getByText(/manage/i))
        expect(mockRouter.push).toHaveBeenCalledWith("/manage")
***REMOVED***)

    test("test logout page redirection", async () => {
        const mockRouter = {
            push: jest.fn() // the component uses `router.push` only
***REMOVED***;
        // make useRouter return an object with a fake push function in it
        (useRouter as jest.Mock).mockReturnValue(mockRouter)
        const {getByText***REMOVED*** = render(<Navbar/>);

        await userEvent.click(getByText(/logout/i))
        expect(mockRouter.push).toHaveBeenCalledWith("/login")
***REMOVED***)
***REMOVED***)